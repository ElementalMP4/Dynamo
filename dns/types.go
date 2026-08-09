package dns

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type DNSRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int    `json:"ttl"`

	IPv4  string   `json:"ipv4,omitempty"`
	CNAME string   `json:"cname,omitempty"`
	TXT   []string `json:"txt,omitempty"`

	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
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

// UnmarshalJSON accepts both legacy numeric IDs and UUID string IDs.
func (r *DNSRecord) UnmarshalJSON(data []byte) error {
	type wireRecord struct {
		ID       any      `json:"id"`
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		TTL      int      `json:"ttl"`
		IPv4     string   `json:"ipv4,omitempty"`
		CNAME    string   `json:"cname,omitempty"`
		TXT      []string `json:"txt,omitempty"`
		Priority int      `json:"priority"`
		Weight   int      `json:"weight"`
		Port     int      `json:"port,omitempty"`
		Target   string   `json:"target,omitempty"`
	}

	var wire wireRecord
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch id := wire.ID.(type) {
	case string:
		r.ID = id
	case float64:
		r.ID = strconv.FormatInt(int64(id), 10)
	case nil:
		r.ID = ""
	default:
		return fmt.Errorf("unsupported dns record id type: %T", wire.ID)
	}

	r.Name = wire.Name
	r.Type = wire.Type
	r.TTL = wire.TTL
	r.IPv4 = wire.IPv4
	r.CNAME = wire.CNAME
	r.TXT = wire.TXT
	r.Priority = wire.Priority
	r.Weight = wire.Weight
	r.Port = wire.Port
	r.Target = wire.Target

	return nil
}
