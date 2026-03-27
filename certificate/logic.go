package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"Dynamo/config"
	"Dynamo/types"

	"github.com/ElementalMP4/loggerino/log"
	"software.sslmate.com/src/go-pkcs12"
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

func listAllCerts() (types.CertListing, error) {
	base := certsBaseDir()
	kinds := []string{"root", "server", "client"}

	out := make(types.CertListing)

	for _, kind := range kinds {
		dir := filepath.Join(base, kind)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("listing %s certs: %w", kind, err)
		}

		var kindCerts []types.CertDir
		for _, entry := range entries {
			subDir := filepath.Join(dir, entry.Name())

			certFiles, err := os.ReadDir(subDir)
			if err != nil {
				return nil, fmt.Errorf("listing cert dir %s/%s: %w", kind, entry.Name(), err)
			}

			files := make([]string, 0, len(certFiles))
			for _, f := range certFiles {
				files = append(files, f.Name())
			}

			kindCerts = append(kindCerts, types.CertDir{
				Name:  entry.Name(),
				Files: files,
			})
		}

		out[kind] = kindCerts
	}

	return out, nil
}

func CountCerts() (map[string]int, error) {
	certs, err := listAllCerts()
	if err != nil {
		return nil, err
	}

	out := make(map[string]int)
	for kind, dirs := range certs {
		out[kind] = len(dirs)
	}

	return out, nil
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

func deleteCertGroup(kind, base string) error {
	root := certsBaseDir()
	dirToDelete := filepath.Join(root, kind, base)

	err := os.RemoveAll(dirToDelete)
	if err != nil {
		return err
	}

	log.Okf("certificates", "Deleted %s/%s", kind, base)
	return nil
}

func createClientCert(name, identity, caBase string, days int, p12Pwd string) (crtPath, keyPath, p12Path, warning string, err error) {
	if err := ensureCertDirs("client", name); err != nil {
		return "", "", "", "", err
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", caBase, caBase+".key")
	caCrtPath := filepath.Join(base, "root", caBase, caBase+".crt")

	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read CA cert: %w", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read CA key: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return "", "", "", "", errors.New("invalid CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return "", "", "", "", err
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return "", "", "", "", errors.New("invalid CA key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return "", "", "", "", err
	}

	// generate client key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", "", err
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", "", "", err
	}

	now := time.Now()
	subject := pkix.Name{CommonName: name}

	subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
		Type:  []int{2, 5, 4, 42}, // OID for givenName
		Value: identity,
	})

	subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
		Type:  []int{1, 2, 840, 113549, 1, 9, 1}, // OID for emailAddress
		Value: identity,
	})

	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      subject,
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(0, 0, days),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return "", "", "", "", err
	}

	clientDir := filepath.Join(base, "client")
	fileBase := name
	crtPath = filepath.Join(clientDir, fileBase, fileBase+".crt")
	keyPath = filepath.Join(clientDir, fileBase, fileBase+".key")
	p12Path = filepath.Join(clientDir, fileBase, fileBase+".p12")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		return "", "", "", "", err
	}

	// create p12 bundle using pkcs12 encoder
	clientCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse client cert: %w", err)
	}

	// Try the legacy encoder first for maximum compatibility (macOS Keychain
	// historically expects older algorithm choices). If that fails, fall back
	// to the Modern encoder. If both succeed, write the legacy .p12 to the
	// canonical filename and the modern one alongside with a .modern.p12 suffix.
	var written bool

	// attempt legacy encode (provides wider compatibility with older importers)
	p12Data, err := pkcs12.Legacy.Encode(priv, clientCert, []*x509.Certificate{caCert}, p12Pwd)
	if err == nil {
		if err := os.WriteFile(p12Path, p12Data, 0o600); err != nil {
			return "", "", "", "", err
		}
		written = true
	} else {
		log.Errorf("certificates", "legacy pkcs12 encode failed: %v", err)
	}

	// try modern encoder as a fallback and also to provide a stronger bundle
	modernData, merr := pkcs12.Modern.Encode(priv, clientCert, []*x509.Certificate{caCert}, p12Pwd)
	if merr == nil {
		// if legacy already wrote the canonical file, write modern as a sibling
		if written {
			modernPath := filepath.Join(clientDir, fileBase, fileBase+".modern.p12")
			_ = os.WriteFile(modernPath, modernData, 0o600)
		} else {
			// write modern to the canonical path if legacy failed
			if err := os.WriteFile(p12Path, modernData, 0o600); err != nil {
				return "", "", "", "", err
			}
			written = true
		}
	} else {
		log.Errorf("certificates", "modern pkcs12 encode failed: %v", merr)
	}

	if !written {
		log.Errorf("certificates", "pkcs12 encode failed (both legacy and modern)")
		warning = "pkcs12 encode failed; .p12 not created"
		p12Path = ""
	}

	return crtPath, keyPath, p12Path, warning, nil
}

func createRootCA(name string, days int) (crtPath, keyPath string, err error) {
	if err := ensureCertDirs("root", name); err != nil {
		return "", "", err
	}

	// generate key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", err
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	base := certsBaseDir()
	rootDir := filepath.Join(base, "root")
	crtPath = filepath.Join(rootDir, name+"/"+name+".crt")
	keyPath = filepath.Join(rootDir, name+"/"+name+".key")

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		return "", "", err
	}

	return crtPath, keyPath, nil
}

func createServerCert(domain, caBase string, days int) (crtPath, keyPath, nginxMtls, nginxRedirect string, err error) {
	if err := ensureCertDirs("server", domain); err != nil {
		return "", "", "", "", err
	}

	base := certsBaseDir()
	caKeyPath := filepath.Join(base, "root", caBase, caBase+".key")
	caCrtPath := filepath.Join(base, "root", caBase, caBase+".crt")

	// load CA
	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read CA cert: %w", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read CA key: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return "", "", "", "", errors.New("invalid CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return "", "", "", "", err
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return "", "", "", "", errors.New("invalid CA key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return "", "", "", "", err
	}

	// generate server key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", "", err
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", "", "", err
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: domain},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{domain},
		BasicConstraintsValid: true,
		IsCA:                  false,
		AuthorityKeyId:        caCert.SubjectKeyId,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return "", "", "", "", err
	}

	serverDir := filepath.Join(base, "server")
	fileBase := domain
	crtPath = filepath.Join(serverDir, fileBase, fileBase+".crt")
	keyPath = filepath.Join(serverDir, fileBase, fileBase+".key")

	crtAbs, _ := filepath.Abs(crtPath)
	keyAbs, _ := filepath.Abs(keyPath)
	caAbs, _ := filepath.Abs(caCrtPath)

	nginxMtlsPath := filepath.Join(serverDir, fileBase, "nginx-mtls.conf")
	nginxMtls, _ = filepath.Abs(nginxMtlsPath)

	nginxRedirectPath := filepath.Join(serverDir, fileBase, "nginx-redirect.conf")
	nginxRedirect, _ = filepath.Abs(nginxRedirectPath)

	if err := writeNginxMTLSConfig(
		domain,
		crtAbs,
		keyAbs,
		caAbs,
		nginxMtls,
	); err != nil {
		return "", "", "", "", fmt.Errorf("failed to write nginx mTLS config: %w", err)
	}

	if err := writeNginxRedirectConfig(
		domain,
		nginxRedirect,
	); err != nil {
		return "", "", "", "", fmt.Errorf("failed to write nginx redirect config: %w", err)
	}

	if err := os.WriteFile(crtPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o644); err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600); err != nil {
		return "", "", "", "", err
	}

	return crtAbs, keyAbs, nginxMtls, nginxRedirect, nil
}
