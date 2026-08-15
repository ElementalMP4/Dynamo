package api

type InterfaceStats struct {
	IPs []string `json:"ip"`
	RX  uint64   `json:"rx"`
	TX  uint64   `json:"tx"`
}

type CPUStats struct {
	User    uint64 `json:"user"`
	Nice    uint64 `json:"nice"`
	System  uint64 `json:"system"`
	Idle    uint64 `json:"idle"`
	Iowait  uint64 `json:"iowait"`
	Irq     uint64 `json:"irq"`
	Softirq uint64 `json:"softirq"`
	Steal   uint64 `json:"steal"`
}

type MemStats struct {
	TotalKB     uint64 `json:"total_kb"`
	AvailableKB uint64 `json:"available_kb"`
}

type SystemStats struct {
	CPU CPUStats                  `json:"cpu"`
	Mem MemStats                  `json:"mem"`
	Net map[string]InterfaceStats `json:"net"`
}
