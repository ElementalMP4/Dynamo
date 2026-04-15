package vpn

import "sync"

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
	RxBps         uint64 `json:"rx_bps"`
	TxBps         uint64 `json:"tx_bps"`
	LastHandshake int64  `json:"last_handshake"`
}

type InterfaceStats struct {
	Name  string      `json:"name"`
	RxBps uint64      `json:"rx_bps"`
	TxBps uint64      `json:"tx_bps"`
	Peers []PeerStats `json:"peers"`
}

type peerMetrics struct {
	RxBytes  uint64
	TxBytes  uint64
	LastSeen int64
}

type ifaceMetrics struct {
	Peers map[string]peerMetrics
}

var metricsCache = struct {
	mu   sync.Mutex
	data map[string]ifaceMetrics
}{
	data: make(map[string]ifaceMetrics),
}
