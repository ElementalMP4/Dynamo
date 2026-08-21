package dns

import (
	"fmt"
	"strings"
)

type DNSRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int    `json:"ttl"`

	IPv4  string   `json:"ipv4,omitempty"`
	CNAME string   `json:"cname,omitempty"`
	TXT   []string `json:"txt,omitempty"`

	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Target   string `json:"target,omitempty"`
}

// DNSRecordPatch tracks only fields explicitly provided by clients.
// This avoids accidental zero-value overwrites during partial updates.
type DNSRecordPatch struct {
	Name     *string   `json:"name,omitempty"`
	Type     *string   `json:"type,omitempty"`
	TTL      *int      `json:"ttl,omitempty"`
	IPv4     *string   `json:"ipv4,omitempty"`
	CNAME    *string   `json:"cname,omitempty"`
	TXT      *[]string `json:"txt,omitempty"`
	Priority *int      `json:"priority,omitempty"`
	Weight   *int      `json:"weight,omitempty"`
	Port     *int      `json:"port,omitempty"`
	Target   *string   `json:"target,omitempty"`
}

func (r *DNSRecord) ToTxtResponse() string {
	return strings.Join(r.TXT, ", ")
}

func (r *DNSRecord) NameTrailingDot() string {
	return r.Name + "."
}

func (r *DNSRecord) ToSrvResponse() string {
	return fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, r.Target)
}
