package api

import (
	"Dynamo/config"
	"Dynamo/util"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func fetchRemoteCertCounts(cfg config.Config) (map[string]int, error) {
	remote, err := url.Parse(cfg.Certificates.RemoteCertURL)
	if err != nil {
		return nil, fmt.Errorf("invalid remote certificate URL: %w", err)
	}

	statsURL := remote.ResolveReference(&url.URL{Path: "/api/stats/dynamo"})

	req, err := http.NewRequest(http.MethodGet, statsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating remote stats request: %w", err)
	}

	resp, err := util.GetRemoteHttpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting remote stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote stats request failed: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Certs map[string]int `json:"certs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding remote stats response: %w", err)
	}

	return payload.Certs, nil
}
