package dns

import (
	"Dynamo/config"
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/miekg/dns"
)

const (
	upstreamTimeout     = 2 * time.Second
	upstreamIdleTimeout = 30 * time.Second
	upstreamPoolSize    = 8
)

type upstreamCacheEntry struct {
	msg     *dns.Msg
	stored  time.Time
	expires time.Time
}

type upstreamFlight struct {
	done chan struct{}
	msg  *dns.Msg
	err  error
}

type upstreamResolver struct {
	mu      sync.Mutex
	cache   map[string]upstreamCacheEntry
	flights map[string]*upstreamFlight
	servers map[config.UpstreamServer]*upstreamServer
}

var externalResolver = newUpstreamResolver()

func newUpstreamResolver() *upstreamResolver {
	return &upstreamResolver{
		cache:   make(map[string]upstreamCacheEntry),
		flights: make(map[string]*upstreamFlight),
		servers: make(map[config.UpstreamServer]*upstreamServer),
	}
}

func upstreamQueryKey(req *dns.Msg) (string, bool, error) {
	// Signed, non-query and compound messages must not share transaction state.
	if req.Opcode != dns.OpcodeQuery || req.Response || req.Truncated ||
		len(req.Question) != 1 || len(req.Answer) != 0 || len(req.Ns) != 0 {
		return "", false, nil
	}
	if len(req.Extra) > 1 || (len(req.Extra) == 1 && req.Extra[0].Header().Rrtype != dns.TypeOPT) {
		return "", false, nil
	}
	key := req.Copy()
	key.Id = 0
	key.Compress = false
	key.Question[0].Name = strings.ToLower(key.Question[0].Name)
	wire, err := key.Pack()
	if err != nil {
		return "", false, fmt.Errorf("encode upstream query key: %w", err)
	}
	// The wire key includes numeric type/class, flags, and all EDNS options.
	return string(wire), true, nil
}

func upstreamReply(resp, req *dns.Msg) *dns.Msg {
	reply := resp.Copy()
	reply.Id = req.Id
	reply.Question = append([]dns.Question(nil), req.Question...)
	return reply
}

func positiveTTL(resp *dns.Msg) uint32 {
	if resp.Rcode != dns.RcodeSuccess || resp.Truncated || len(resp.Answer) == 0 || resp.IsTsig() != nil {
		return 0
	}
	min := ^uint32(0)
	for _, section := range [][]dns.RR{resp.Answer, resp.Ns, resp.Extra} {
		for _, rr := range section {
			if rr.Header().Rrtype != dns.TypeOPT && rr.Header().Ttl < min {
				min = rr.Header().Ttl
			}
		}
	}
	return min
}

func ageUpstreamTTLs(resp *dns.Msg, elapsed uint32) {
	for _, section := range [][]dns.RR{resp.Answer, resp.Ns, resp.Extra} {
		for _, rr := range section {
			// OPT's TTL field carries EDNS flags and version, not a lifetime.
			if rr.Header().Rrtype == dns.TypeOPT {
				continue
			}
			if rr.Header().Ttl > elapsed {
				rr.Header().Ttl -= elapsed
			} else {
				rr.Header().Ttl = 0
			}
		}
	}
}

func (r *upstreamResolver) resolve(req *dns.Msg, upstreams []config.UpstreamServer) (*dns.Msg, error) {
	key, shareable, err := upstreamQueryKey(req)
	if err != nil {
		return nil, err
	}
	if !shareable {
		return r.forward(req, upstreams)
	}

	r.mu.Lock()
	if entry, ok := r.cache[key]; ok {
		if time.Now().Before(entry.expires) {
			r.mu.Unlock()
			reply := upstreamReply(entry.msg, req)
			ageUpstreamTTLs(reply, uint32(time.Since(entry.stored)/time.Second))
			return reply, nil
		}
		delete(r.cache, key)
	}
	if flight, ok := r.flights[key]; ok {
		r.mu.Unlock()
		<-flight.done
		if flight.err != nil {
			return nil, flight.err
		}
		return upstreamReply(flight.msg, req), nil
	}
	flight := &upstreamFlight{done: make(chan struct{})}
	r.flights[key] = flight
	r.mu.Unlock()

	resp, err := r.forward(req, upstreams)
	r.mu.Lock()
	if err == nil {
		if ttl := positiveTTL(resp); ttl > 0 {
			now := time.Now()
			r.cache[key] = upstreamCacheEntry{
				msg: resp.Copy(), stored: now, expires: now.Add(time.Duration(ttl) * time.Second),
			}
		}
	}
	flight.msg, flight.err = resp, err
	delete(r.flights, key)
	close(flight.done)
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return upstreamReply(resp, req), nil
}

type idleUpstreamConn struct {
	conn *dns.Conn
	used time.Time
}

type upstreamServer struct {
	client dns.Client
	slots  chan struct{}

	mu       sync.Mutex
	idle     []idleUpstreamConn
	failures uint
	retryAt  time.Time
}

func (r *upstreamResolver) server(upstream config.UpstreamServer) *upstreamServer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if server, ok := r.servers[upstream]; ok {
		return server
	}
	server := &upstreamServer{
		client: dns.Client{Net: "udp", Timeout: upstreamTimeout},
		slots:  make(chan struct{}, upstreamPoolSize),
	}
	if upstream.UpstreamTLSServerName != "" {
		server.client.Net = "tcp-tls"
		server.client.TLSConfig = &tls.Config{
			ServerName:         upstream.UpstreamTLSServerName,
			MinVersion:         tls.VersionTLS12,
			ClientSessionCache: tls.NewLRUClientSessionCache(upstreamPoolSize),
		}
	}
	r.servers[upstream] = server
	return server
}

func (s *upstreamServer) retryTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryAt
}

func (s *upstreamServer) recordResult(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.failures = 0
		s.retryAt = time.Time{}
		return
	}
	delay := min(time.Second<<s.failures, 30*time.Second)
	if s.failures < 5 {
		s.failures++
	}
	s.retryAt = time.Now().Add(delay)
}

func (s *upstreamServer) takeIdle(now time.Time) *dns.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.idle) > 0 {
		last := len(s.idle) - 1
		idle := s.idle[last]
		s.idle = s.idle[:last]
		if now.Sub(idle.used) < upstreamIdleTimeout {
			return idle.conn
		}
		idle.conn.Close()
	}
	return nil
}

func (s *upstreamServer) exchange(req *dns.Msg, address string) (*dns.Msg, error) {
	ctx, cancel := context.WithTimeout(context.Background(), upstreamTimeout)
	defer cancel()
	if s.client.Net != "tcp-tls" {
		resp, _, err := s.client.ExchangeContext(ctx, req, address)
		return resp, err
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	conn := s.takeIdle(time.Now())
	reused := conn != nil
	if conn == nil {
		var err error
		conn, err = s.client.DialContext(ctx, address)
		if err != nil {
			return nil, err
		}
	}
	resp, _, err := s.client.ExchangeWithConnContext(ctx, req, conn)
	if err == nil && !validUpstreamResponse(req, resp) {
		conn.Close()
		return nil, fmt.Errorf("mismatched upstream response")
	}
	if err != nil {
		conn.Close()
		// A server may close an idle connection; retry once within the same budget.
		if reused && ctx.Err() == nil {
			conn, err = s.client.DialContext(ctx, address)
			if err == nil {
				resp, _, err = s.client.ExchangeWithConnContext(ctx, req, conn)
				if err == nil && !validUpstreamResponse(req, resp) {
					err = fmt.Errorf("mismatched upstream response")
				}
				if err != nil {
					conn.Close()
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.idle = append(s.idle, idleUpstreamConn{conn: conn, used: time.Now()})
	s.mu.Unlock()
	return resp, nil
}

func (r *upstreamResolver) forward(req *dns.Msg, upstreams []config.UpstreamServer) (*dns.Msg, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("no DNS upstreams configured")
	}
	now := time.Now()
	available := make([]config.UpstreamServer, 0, len(upstreams))
	earliest := upstreams[0]
	earliestRetry := r.server(earliest).retryTime()
	for _, upstream := range upstreams {
		retry := r.server(upstream).retryTime()
		if !now.Before(retry) {
			available = append(available, upstream)
		}
		if retry.Before(earliestRetry) {
			earliest, earliestRetry = upstream, retry
		}
	}
	// If every upstream is cooling down, probe the earliest rather than refusing all queries.
	if len(available) == 0 {
		available = append(available, earliest)
	}

	var lastErr error
	for _, upstream := range available {
		server := r.server(upstream)
		resp, err := server.exchange(req, upstream.DialAddress)
		if err == nil && !validUpstreamResponse(req, resp) {
			err = fmt.Errorf("mismatched upstream response")
		}
		if err == nil && (resp.Rcode == dns.RcodeServerFailure || resp.Rcode == dns.RcodeRefused) {
			err = fmt.Errorf("returned %s", dns.RcodeToString[resp.Rcode])
		}
		server.recordResult(err)
		if err == nil {
			return resp, nil
		}
		lastErr = fmt.Errorf("%s: %w", upstream.DialAddress, err)
		log.Warnf(loggingSource, "DNS upstream failed: %v", lastErr)
	}
	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func validUpstreamResponse(req, resp *dns.Msg) bool {
	if resp == nil || !resp.Response || resp.Id != req.Id || resp.Opcode != req.Opcode ||
		len(resp.Question) != len(req.Question) {
		return false
	}
	for i, q := range req.Question {
		answer := resp.Question[i]
		if !strings.EqualFold(q.Name, answer.Name) || q.Qtype != answer.Qtype || q.Qclass != answer.Qclass {
			return false
		}
	}
	return true
}

func (r *upstreamResolver) cleanup(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, entry := range r.cache {
		if !now.Before(entry.expires) {
			delete(r.cache, key)
		}
	}
	for _, server := range r.servers {
		server.mu.Lock()
		kept := server.idle[:0]
		for _, idle := range server.idle {
			if now.Sub(idle.used) >= upstreamIdleTimeout {
				idle.conn.Close()
			} else {
				kept = append(kept, idle)
			}
		}
		server.idle = kept
		server.mu.Unlock()
	}
}
