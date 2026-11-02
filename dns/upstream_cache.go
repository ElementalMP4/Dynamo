package dns

import (
	"Dynamo/util"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
)

const upstreamCachePageSize = 50

type upstreamCacheRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Class      string    `json:"class"`
	TTL        uint32    `json:"ttl"`
	CachedAt   time.Time `json:"cached_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Answers    []string  `json:"answers"`
	Authority  []string  `json:"authority"`
	Additional []string  `json:"additional"`
}

type upstreamCachePage struct {
	Records    []upstreamCacheRecord `json:"records"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	Total      int                   `json:"total"`
	TotalPages int                   `json:"total_pages"`
}

func (r *upstreamResolver) listCache(page int) upstreamCachePage {
	now := time.Now()
	r.mu.Lock()
	entries := make([]upstreamCacheEntry, 0, len(r.cache))
	for key, entry := range r.cache {
		if !now.Before(entry.expires) {
			delete(r.cache, key)
			continue
		}
		entries = append(entries, entry)
	}
	r.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i].msg.Question[0], entries[j].msg.Question[0]
		if nameLeft, nameRight := strings.ToLower(left.Name), strings.ToLower(right.Name); nameLeft != nameRight {
			return nameLeft < nameRight
		}
		if left.Qtype != right.Qtype {
			return left.Qtype < right.Qtype
		}
		if left.Qclass != right.Qclass {
			return left.Qclass < right.Qclass
		}
		return entries[i].id < entries[j].id
	})
	total := len(entries)
	totalPages := (total + upstreamCachePageSize - 1) / upstreamCachePageSize
	page = max(1, min(page, totalPages))
	start := (page - 1) * upstreamCachePageSize
	result := upstreamCachePage{
		Records: make([]upstreamCacheRecord, 0, upstreamCachePageSize),
		Page:    page, PageSize: upstreamCachePageSize, Total: total, TotalPages: totalPages,
	}
	for _, entry := range entries[start:min(start+upstreamCachePageSize, total)] {
		msg := entry.msg.Copy()
		ageUpstreamTTLs(msg, uint32(now.Sub(entry.stored)/time.Second))
		q := msg.Question[0]
		result.Records = append(result.Records, upstreamCacheRecord{
			ID: entry.id, Name: q.Name, Type: dns.Type(q.Qtype).String(), Class: dns.Class(q.Qclass).String(),
			TTL:      uint32((entry.expires.Sub(now) + time.Second - 1) / time.Second),
			CachedAt: entry.stored, ExpiresAt: entry.expires,
			Answers: upstreamRecordStrings(msg.Answer), Authority: upstreamRecordStrings(msg.Ns),
			Additional: upstreamRecordStrings(msg.Extra),
		})
	}
	return result
}

func upstreamRecordStrings(records []dns.RR) []string {
	result := make([]string, 0, len(records))
	for _, record := range records {
		result = append(result, record.String())
	}
	return result
}

func (r *upstreamResolver) clearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.cache)
	for _, flight := range r.flights {
		flight.discardCache = true
	}
}

func (r *upstreamResolver) deleteCachedRecord(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, entry := range r.cache {
		if entry.id == id {
			delete(r.cache, key)
			if flight, ok := r.flights[key]; ok {
				flight.discardCache = true
			}
			return time.Now().Before(entry.expires)
		}
	}
	return false
}

func ListUpstreamCache(c *gin.Context) {
	page := 1
	if value, provided := c.GetQuery("page"); provided {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmt.Errorf("page must be a positive integer"))
			return
		}
		page = parsed
	}
	c.JSON(http.StatusOK, externalResolver.listCache(page))
}

func ClearUpstreamCache(c *gin.Context) {
	externalResolver.clearCache()
	log.Info(loggingSource, "Cleared upstream DNS cache")
	c.Status(http.StatusNoContent)
}

func DeleteUpstreamCacheRecord(c *gin.Context) {
	id, err := parseRecordID(c)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}
	if !externalResolver.deleteCachedRecord(id) {
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmt.Errorf("cached DNS record %s not found", id))
		return
	}
	log.Infof(loggingSource, "Cleared upstream DNS cache record %s", id)
	c.Status(http.StatusNoContent)
}
