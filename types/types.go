package types

type DNSRecord struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   int    `json:"ttl"`
	Class string `json:"class"`

	IPv4  *string  `json:"ipv4,omitempty"`
	CNAME *string  `json:"cname,omitempty"`
	TXT   []string `json:"txt,omitempty"`

	Priority *int    `json:"priority,omitempty"`
	Weight   *int    `json:"weight,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Target   *string `json:"target,omitempty"`
}

type Config struct {
	ServerAddress string `json:"serverBind"`
	CertsDir      string `json:"certsDir"`
}
