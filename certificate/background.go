package certificate

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ElementalMP4/loggerino/log"
)

func refreshCrls() ([]string, error) {
	remoteCerts, err := FetchRemoteCerts()
	if err != nil {
		return nil, err
	}

	processedRootCerts := []string{}

	roots := remoteCerts["root"]
	for _, root := range roots {
		hasRoot, err := hasRootCertificate(root.Name)
		if err != nil {
			return nil, err
		}

		if !hasRoot {
			continue
		}

		crlFile, err := FetchRemoteCertFile("root", root.Name, root.Name+".crl")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(certsBaseDir(), "root", root.Name, root.Name+".crl")
		if err := os.WriteFile(path, crlFile, 0o644); err != nil {
			return nil, err
		}

		processedRootCerts = append(processedRootCerts, root.Name)
	}

	return processedRootCerts, nil
}

func updateWrapper() {
	updated, err := refreshCrls()
	if err != nil {
		log.Errorf("certs/crl-fetch", "Failed to refresh CRLs: %v", err)
	}
	log.Okf("certs/crl-fetch", "Successfully updated CRLs: %s", strings.Join(updated, ", "))
}

func StartCrlRefreshLoop() {
	updateTicker := time.NewTicker(15 * time.Minute)
	updateWrapper()
	for range updateTicker.C {
		updateWrapper()
	}
}
