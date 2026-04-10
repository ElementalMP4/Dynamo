package certificate

type CertDir struct {
	Name  string   `json:"name,omitempty"`
	Files []string `json:"files"`
}

type CertListing map[string][]CertDir

type CertMetadata struct {
	CaBase       string `json:"caBase"`
	SerialNumber string `json:"serialNumber"`
	Type         string `json:"type"`
}
