package dns

type DNSRecord struct {
	ID   int    `json:"id"`
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
