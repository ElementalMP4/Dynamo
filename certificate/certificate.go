package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TinyVTM/config"
	"TinyVTM/log"

	"github.com/gin-gonic/gin"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// helper: return configured certs base dir or default
func certsBaseDir() string {
	cfg := config.GetConfig()
	if cfg.CertsDir != "" {
		return cfg.CertsDir
	}
	return "./certs"
}

// ensureCertDirs makes sure root, server and client subdirs exist
func ensureCertDirs() error {
	base := certsBaseDir()
	dirs := []string{
		filepath.Join(base, "root"),
		filepath.Join(base, "server"),
		filepath.Join(base, "client"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// POST /api/certs/server
// body: {"domain":"example.com","caBase":"rootCA","days":3650}
func CreateServerCert(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
		CaBase string `json:"caBase"`
		Days   int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain required"})
		return
	}
	if req.CaBase == "" {
		req.CaBase = "rootCA"
	}
	if req.Days == 0 {
		req.Days = 3650
	}

	if err := ensureCertDirs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", req.CaBase+".key")
	caCrtPath := filepath.Join(base, "root", req.CaBase+".crt")

	// load CA
	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read CA cert: " + err.Error()})
		return
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read CA key: " + err.Error()})
		return
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid CA cert PEM"})
		return
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid CA key PEM"})
		return
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// generate server key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: req.Domain},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, req.Days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{req.Domain},
		BasicConstraintsValid: true,
		IsCA:                  false,
		AuthorityKeyId:        caCert.SubjectKeyId,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, caCert, &priv.PublicKey, caKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	serverDir := filepath.Join(base, "server")
	fileBase := req.Domain
	crtPath := filepath.Join(serverDir, fileBase+".crt")
	keyPath := filepath.Join(serverDir, fileBase+".key")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.LogInfo(fmt.Sprintf("Created server cert: %s and %s", crtPath, keyPath))
	c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath})
}

// POST /api/certs/root
// body: {"name":"rootCA","days":3650}
func CreateRootCA(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Days int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = "rootCA"
	}
	if req.Days == 0 {
		req.Days = 3650
	}

	if err := ensureCertDirs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// generate key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: req.Name},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, req.Days),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	base := certsBaseDir()
	rootDir := filepath.Join(base, "root")
	fileBase := req.Name
	crtPath := filepath.Join(rootDir, fileBase+".crt")
	keyPath := filepath.Join(rootDir, fileBase+".key")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.LogInfo(fmt.Sprintf("Created root CA: %s and %s", crtPath, keyPath))
	c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath})
}

// POST /api/certs/client
// body: {"name":"alice","caBase":"rootCA","days":3650, "p12Pass":"secret"}
func CreateClientCert(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		CaBase string `json:"caBase"`
		Days   int    `json:"days"`
		P12Pwd string `json:"p12Pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if req.CaBase == "" {
		req.CaBase = "rootCA"
	}
	if req.Days == 0 {
		req.Days = 3650
	}

	if err := ensureCertDirs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", req.CaBase+".key")
	caCrtPath := filepath.Join(base, "root", req.CaBase+".crt")

	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read CA cert: " + err.Error()})
		return
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read CA key: " + err.Error()})
		return
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid CA cert PEM"})
		return
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid CA key PEM"})
		return
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// generate client key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: req.Name},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(0, 0, req.Days),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, caCert, &priv.PublicKey, caKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	clientDir := filepath.Join(base, "client")
	fileBase := req.Name
	crtPath := filepath.Join(clientDir, fileBase+".crt")
	keyPath := filepath.Join(clientDir, fileBase+".key")
	p12Path := filepath.Join(clientDir, fileBase+".p12")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// create p12 bundle using pure-Go pkcs12 encoder
	p12pwd := req.P12Pwd
	clientCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse client cert: " + err.Error()})
		return
	}

	// Try the legacy encoder first for maximum compatibility (macOS Keychain
	// historically expects older algorithm choices). If that fails, fall back
	// to the Modern encoder. If both succeed, write the legacy .p12 to the
	// canonical filename and the modern one alongside with a .modern.p12 suffix.
	var written bool
	// attempt legacy encode (provides wider compatibility with older importers)
	p12Data, err := pkcs12.Legacy.Encode(priv, clientCert, []*x509.Certificate{caCert}, p12pwd)
	if err == nil {
		if err := os.WriteFile(p12Path, p12Data, 0o600); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		written = true
	} else {
		log.LogInfo(fmt.Sprintf("legacy pkcs12 encode failed: %v", err))
	}

	// try modern encoder as a fallback and also to provide a stronger bundle
	modernData, merr := pkcs12.Modern.Encode(priv, clientCert, []*x509.Certificate{caCert}, p12pwd)
	if merr == nil {
		// if legacy already wrote the canonical file, write modern as a sibling
		if written {
			modernPath := filepath.Join(clientDir, fileBase+".modern.p12")
			_ = os.WriteFile(modernPath, modernData, 0o600)
		} else {
			// write modern to the canonical path if legacy failed
			if err := os.WriteFile(p12Path, modernData, 0o600); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			written = true
		}
	} else {
		log.LogInfo(fmt.Sprintf("modern pkcs12 encode failed: %v", merr))
	}

	if !written {
		log.LogInfo("pkcs12 encode failed (both legacy and modern)")
		c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath, "p12": nil, "warning": "pkcs12 encode failed; .p12 not created"})
		return
	}

	log.LogInfo(fmt.Sprintf("Created client cert: %s, %s, %s", crtPath, keyPath, p12Path))
	c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath, "p12": p12Path})
}

// GET /api/certs/list
// returns a JSON map of kind->[]files
func ListCerts(c *gin.Context) {
	base := certsBaseDir()
	kinds := []string{"root", "server", "client"}
	type CertFile struct {
		Name string `json:"name"`
		Ext  string `json:"ext"`
	}
	type CertGroup struct {
		Base       string     `json:"base"`
		CommonName string     `json:"cn,omitempty"`
		NotBefore  time.Time  `json:"notBefore,omitempty"`
		NotAfter   time.Time  `json:"notAfter,omitempty"`
		Files      []CertFile `json:"files"`
	}

	out := map[string][]CertGroup{}
	for _, k := range kinds {
		dir := filepath.Join(base, k)
		entries, err := os.ReadDir(dir)
		if err != nil {
			out[k] = []CertGroup{}
			continue
		}
		groups := map[string]*CertGroup{}
		// iterate files and group by sanitized base
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			g, ok := groups[name]
			if !ok {
				g = &CertGroup{Base: name, Files: []CertFile{}}
				groups[name] = g
			}
			ext := strings.ToLower(filepath.Ext(name))
			g.Files = append(g.Files, CertFile{Name: name, Ext: ext})

			// if this file looks like a cert, try to parse to extract CN and validity
			if ext == ".crt" || ext == ".pem" {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err == nil {
					var block *pem.Block
					rest := data
					for {
						block, rest = pem.Decode(rest)
						if block == nil {
							break
						}
						if block.Type == "CERTIFICATE" {
							cert, err := x509.ParseCertificate(block.Bytes)
							if err == nil {
								// set CN and validity if not already set
								if g.CommonName == "" {
									g.CommonName = cert.Subject.CommonName
								}
								if g.NotBefore.IsZero() {
									g.NotBefore = cert.NotBefore
								}
								if g.NotAfter.IsZero() {
									g.NotAfter = cert.NotAfter
								}
								break
							}
						}
					}
				}
			}
		}
		// convert map to slice preserving insertion-ish order
		var list []CertGroup
		for _, g := range groups {
			list = append(list, *g)
		}
		out[k] = list
	}
	c.JSON(http.StatusOK, out)
}

// GET /api/certs/download?kind=client&name=alice_client.p12
func DownloadCert(c *gin.Context) {
	kind := c.Query("kind")
	name := c.Query("name")
	if kind == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind and name required"})
		return
	}
	base := certsBaseDir()
	path := filepath.Join(base, kind, filepath.Clean(name))
	// Prevent directory traversal by ensuring path has expected prefix
	absBase, err := filepath.Abs(filepath.Join(base, kind))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(absPath) < len(absBase) || absPath[:len(absBase)] != absBase {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file"})
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(absPath, filepath.Base(absPath))
}

// DELETE /api/certs?kind=client&base=alice
// Moves files for the given cert base into a revoked directory and creates a revocation metadata file.
func DeleteCertGroup(c *gin.Context) {
	kind := c.Query("kind")
	base := c.Query("base")
	if kind == "" || base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind and base required"})
		return
	}

	root := certsBaseDir()
	srcDir := filepath.Join(root, kind)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	revokedDir := filepath.Join(root, "revoked", kind, base)
	if err := os.MkdirAll(revokedDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	moved := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(revokedDir, name)
		// try rename first
		if err := os.Rename(srcPath, dstPath); err != nil {
			// fallback to copy
			in, err := os.ReadFile(srcPath)
			if err != nil {
				continue
			}
			if err := os.WriteFile(dstPath, in, 0o600); err != nil {
				continue
			}
			_ = os.Remove(srcPath)
		}
		moved++
	}

	// write revocation metadata
	revMeta := fmt.Sprintf("{\"base\":%q,\"kind\":%q,\"revokedAt\":%q}", base, kind, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(revokedDir, "revocation.json"), []byte(revMeta), 0o644)

	if moved == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no matching files found"})
		return
	}
	log.LogInfo(fmt.Sprintf("Revoked %d files for %s/%s", moved, kind, base))
	c.Status(http.StatusNoContent)
}
