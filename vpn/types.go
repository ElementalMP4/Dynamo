package vpn

const inactiveTimeout int64 = 60 * 1000

type Peer struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	PublicKey  string  `json:"public_key"`
	PrivateKey string  `json:"private_key"`
	IP         string  `json:"ip"`
	Enabled    bool    `json:"enabled"`
	DNS        *string `json:"dns,omitempty"`
	AllowedIPs *string `json:"allowed_ips,omitempty"`
}

type Interface struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	ListenPort int    `json:"listen_port"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	Enabled    bool   `json:"enabled"`

	EnableNAT    bool   `json:"enable_nat"`
	OutInterface string `json:"out_interface,omitempty"`

	DefaultDNS        string `json:"default_dns"`
	DefaultAllowedIPs string `json:"default_allowed_ips"`
	Endpoint          string `json:"endpoint"`

	Peers []Peer `json:"peers"`
}

type State struct {
	Interfaces []Interface `json:"interfaces"`
}

func (s *State) ContainsInterface(name string) bool {
	for _, iface := range s.Interfaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}

type CreateInterfaceRequest struct {
	Name       string `json:"name" binding:"required"`
	Address    string `json:"address" binding:"required"`
	ListenPort int    `json:"listen_port" binding:"required"`

	EnableNAT    bool   `json:"enable_nat"`
	OutInterface string `json:"out_interface"`

	DefaultDNS        string `json:"default_dns"`
	DefaultAllowedIPs string `json:"default_allowed_ips"`
	Endpoint          string `json:"endpoint" binding:"required"`
}

type UpdateInterfaceRequest struct {
	DefaultDNS        *string `json:"default_dns"`
	DefaultAllowedIPs *string `json:"default_allowed_ips"`
	Endpoint          *string `json:"endpoint"`
	EnableNAT         *bool   `json:"enable_nat"`
	OutInterface      *string `json:"out_interface"`
}

type CreatePeerRequest struct {
	Name       string  `json:"name" binding:"required"`
	DNS        *string `json:"dns"`
	AllowedIPs *string `json:"allowed_ips"`
}

type UpdatePeerRequest struct {
	Name       *string `json:"name"`
	Enabled    *bool   `json:"enabled"`
	DNS        *string `json:"dns"`
	AllowedIPs *string `json:"allowed_ips"`
}

type ToggleInterfaceRequest struct {
	Enabled bool `json:"enabled"`
}

type PeerStats struct {
	ID            string `json:"id"`
	RX            uint64 `json:"rx"`
	TX            uint64 `json:"tx"`
	LastHandshake int64  `json:"handshake"`
	Active        bool   `json:"active"`
}

type InterfaceStats struct {
	Name  string      `json:"name"`
	RX    uint64      `json:"rx"`
	TX    uint64      `json:"tx"`
	Peers []PeerStats `json:"peers"`
}
