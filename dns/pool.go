package dns

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	maxIdleConnsPerUpstream = 4
	idleConnTTL             = 30 * time.Second
)

type pooledConn struct {
	conn    *dns.Conn
	expires time.Time
}

var (
	upstreamPoolMutex sync.Mutex
	upstreamPool      = map[string][]*pooledConn{}
)

func getUpstreamConn(c *dns.Client, upstream string) (conn *dns.Conn, reused bool, err error) {
	upstreamPoolMutex.Lock()
	conns := upstreamPool[upstream]
	now := time.Now()

	for len(conns) > 0 {
		pc := conns[len(conns)-1]
		conns = conns[:len(conns)-1]
		upstreamPool[upstream] = conns

		if pc.expires.After(now) {
			upstreamPoolMutex.Unlock()
			return pc.conn, true, nil
		}
		pc.conn.Close()
	}
	upstreamPoolMutex.Unlock()

	conn, err = c.Dial(upstream)
	return conn, false, err
}

func releaseUpstreamConn(upstream string, conn *dns.Conn) {
	upstreamPoolMutex.Lock()
	if len(upstreamPool[upstream]) >= maxIdleConnsPerUpstream {
		upstreamPoolMutex.Unlock()
		conn.Close()
		return
	}
	upstreamPool[upstream] = append(upstreamPool[upstream], &pooledConn{conn: conn, expires: time.Now().Add(idleConnTTL)})
	upstreamPoolMutex.Unlock()
}

func exchangeViaPool(c *dns.Client, upstream string, req *dns.Msg) (*dns.Msg, error) {
	conn, reused, err := getUpstreamConn(c, upstream)
	if err != nil {
		return nil, err
	}

	resp, _, err := c.ExchangeWithConn(req, conn)
	if err != nil {
		conn.Close()
		if !reused {
			return nil, err
		}

		conn, err = c.Dial(upstream)
		if err != nil {
			return nil, err
		}
		resp, _, err = c.ExchangeWithConn(req, conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	releaseUpstreamConn(upstream, conn)
	return resp, nil
}
