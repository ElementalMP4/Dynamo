package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"Dynamo/config"

	"github.com/gin-gonic/gin"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func certsBaseDir() string {
	cfg := config.GetConfig()
	if cfg.CertsDir != "" {
		return cfg.CertsDir
	}
	return "./certs"
}

func ensureCertDirs(group string, crt string) error {
	base := certsBaseDir()
	dir := filepath.Join(base, group, crt)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return nil
}

func EnsureSslDirs() error {
	base := certsBaseDir()
	dirs := []string{
		filepath.Join(base, "server"),
		filepath.Join(base, "client"),
		filepath.Join(base, "root"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

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

	if err := ensureCertDirs("server", req.Domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", req.CaBase, req.CaBase+".key")
	caCrtPath := filepath.Join(base, "root", req.CaBase, req.CaBase+".crt")

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
	crtPath := filepath.Join(serverDir, fileBase, fileBase+".crt")
	keyPath := filepath.Join(serverDir, fileBase, fileBase+".key")

	crtAbs, _ := filepath.Abs(crtPath)
	keyAbs, _ := filepath.Abs(keyPath)
	caAbs, _ := filepath.Abs(caCrtPath)

	nginxMtlsPath := filepath.Join(serverDir, fileBase, "nginx-mtls.conf")
	nginxMtlsAbs, _ := filepath.Abs(nginxMtlsPath)

	nginxRedirectPath := filepath.Join(serverDir, fileBase, "nginx-redirect.conf")
	nginxRedirectAbs, _ := filepath.Abs(nginxRedirectPath)

	if err := writeNginxMTLSConfig(
		req.Domain,
		crtAbs,
		keyAbs,
		caAbs,
		nginxMtlsAbs,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to write nginx mTLS config: " + err.Error(),
		})
		return
	}

	if err := writeNginxRedirectConfig(
		req.Domain,
		nginxRedirectAbs,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to write nginx redirect config: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Created server cert for %s", req.Domain)

	c.JSON(http.StatusCreated, gin.H{
		"cert":          crtPath,
		"key":           keyPath,
		"nginxMtls":     nginxMtlsAbs,
		"nginxRedirect": nginxRedirectAbs,
	})

}

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

	if err := ensureCertDirs("root", req.Name); err != nil {
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
	crtPath := filepath.Join(rootDir, req.Name+"/"+req.Name+".crt")
	keyPath := filepath.Join(rootDir, req.Name+"/"+req.Name+".key")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Created root CA %s", req.Name)
	c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath})
}

func CreateClientCert(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Identity string `json:"identity"`
		CaBase   string `json:"caBase"`
		Days     int    `json:"days"`
		P12Pwd   string `json:"p12Pass"`
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

	if err := ensureCertDirs("client", req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", req.CaBase, req.CaBase+".key")
	caCrtPath := filepath.Join(base, "root", req.CaBase, req.CaBase+".crt")

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
	subject := pkix.Name{CommonName: req.Name}

	subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
		Type:  []int{2, 5, 4, 42}, // OID for givenName
		Value: req.Identity,
	})

	subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
		Type:  []int{1, 2, 840, 113549, 1, 9, 1}, // OID for emailAddress
		Value: req.Identity,
	})

	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      subject,
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
	crtPath := filepath.Join(clientDir, fileBase, fileBase+".crt")
	keyPath := filepath.Join(clientDir, fileBase, fileBase+".key")
	p12Path := filepath.Join(clientDir, fileBase, fileBase+".p12")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// create p12 bundle using pkcs12 encoder
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
		log.Printf("legacy pkcs12 encode failed: %v", err)
	}

	// try modern encoder as a fallback and also to provide a stronger bundle
	modernData, merr := pkcs12.Modern.Encode(priv, clientCert, []*x509.Certificate{caCert}, p12pwd)
	if merr == nil {
		// if legacy already wrote the canonical file, write modern as a sibling
		if written {
			modernPath := filepath.Join(clientDir, fileBase, fileBase+".modern.p12")
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
		log.Printf("modern pkcs12 encode failed: %v", merr)
	}

	if !written {
		log.Println("pkcs12 encode failed (both legacy and modern)")
		c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath, "p12": nil, "warning": "pkcs12 encode failed; .p12 not created"})
		return
	}

	log.Printf("Created client cert %s", req.Name)
	c.JSON(http.StatusCreated, gin.H{"cert": crtPath, "key": keyPath, "p12": p12Path})
}

func ListCerts(c *gin.Context) {
	base := certsBaseDir()
	kinds := []string{"root", "server", "client"}
	type CertDir struct {
		Name  string   `json:"name,omitempty"`
		Files []string `json:"files"`
	}

	out := map[string][]CertDir{}
	for _, k := range kinds {
		dir := filepath.Join(base, k)
		entries, err := os.ReadDir(dir)

		if err != nil {
			log.Printf("An error occurred whilst listing certificates: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list certs: %v", err)})
			return
		}

		var kindCerts []CertDir
		for _, entry := range entries {
			subDir := filepath.Join(base, k, entry.Name())
			certFiles, err := os.ReadDir(subDir)
			if err != nil {
				log.Printf("An error occurred whilst listing certificates: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list certs: %v", err)})
				return
			}

			var files []string
			for _, certFile := range certFiles {
				files = append(files, certFile.Name())
			}

			found := CertDir{
				Name:  entry.Name(),
				Files: files,
			}

			kindCerts = append(kindCerts, found)
		}

		out[k] = kindCerts

	}
	c.JSON(http.StatusOK, out)
}

func DownloadCert(c *gin.Context) {
	kind := c.Query("kind")
	name := c.Query("name")
	base := c.Query("base")
	if kind == "" || name == "" || base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind, base and name required"})
		return
	}
	certsBase := certsBaseDir()
	path, err := filepath.Abs(filepath.Join(certsBase, kind, base, filepath.Clean(name)))

	if err != nil {
		log.Printf("An error occurred whilst listing certificates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list certs: %v", err)})
		return
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		log.Printf("Certificate file not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Cert file not found: %v", err)})
		return
	}

	c.FileAttachment(path, filepath.Base(path))
}

func DeleteCertGroup(c *gin.Context) {
	kind := c.Query("kind")
	base := c.Query("base")
	if kind == "" || base == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind and base required"})
		return
	}

	root := certsBaseDir()
	dirToDelete := filepath.Join(root, kind, base)

	err := os.RemoveAll(dirToDelete)
	if err != nil {
		panic(err)
	}

	log.Printf("Deleted %s/%s", kind, base)
	c.Status(http.StatusNoContent)
}

func writeNginxMTLSConfig(
	serverName string,
	serverCrt string,
	serverKey string,
	caCrt string,
	outPath string,
) error {

	conf := fmt.Sprintf(`
listen 443 ssl;
server_name %s;

ssl_certificate     %s;
ssl_certificate_key %s;

ssl_client_certificate %s;
ssl_verify_client on;

ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers on;
`, serverName, serverCrt, serverKey, caCrt)

	return os.WriteFile(outPath, []byte(conf), 0o644)
}

func writeNginxRedirectConfig(
	serverName string,
	outPath string,
) error {

	conf := fmt.Sprintf(`
server {
    listen 80;
    server_name %s;

    return 301 https://$host$request_uri;
}
`, serverName)
	return os.WriteFile(outPath, []byte(conf), 0o644)
}
