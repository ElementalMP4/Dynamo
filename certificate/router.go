package certificate

import (
	"Dynamo/config"
	"Dynamo/util"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

const loggingSource string = "cert-router"

func CreateServerCert(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
		CaBase string `json:"caBase"`
		Days   int    `json:"days"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	if req.Domain == "" || req.CaBase == "" {
		fmtErr := fmt.Errorf("domain and caBase values are both required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	if req.Days == 0 {
		req.Days = 3650
	}

	crtPath, keyPath, nginxMtls, nginxRedirect, err := createServerCert(req.Domain, req.CaBase, req.Days)
	if err != nil {
		fmtErr := fmt.Errorf("Failed to create root CA: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Created server cert for %s", req.Domain)
	c.JSON(http.StatusCreated, gin.H{
		"cert":          crtPath,
		"key":           keyPath,
		"nginxMtls":     nginxMtls,
		"nginxRedirect": nginxRedirect,
	})

}

func CreateRootCA(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Days int    `json:"days"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	if req.Name == "" {
		fmtErr := fmt.Errorf("name is required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	if req.Days == 0 {
		req.Days = 3650
	}

	crtPath, keyPath, err := createRootCA(req.Name, req.Days)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	log.Okf(loggingSource, "Created root CA %s", req.Name)
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
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	if req.Name == "" || req.CaBase == "" {
		err := fmt.Errorf("name and caBase are both required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	if req.Days == 0 {
		req.Days = 3650
	}

	crtPath, keyPath, p12Path, err := createClientCert(req.Name, req.Identity, req.CaBase, req.Days, req.P12Pwd)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, err)
		return
	}

	response := gin.H{"cert": crtPath, "key": keyPath}
	if p12Path != "" {
		response["p12"] = p12Path
	} else {
		response["p12"] = nil
	}

	log.Okf(loggingSource, "Created client cert %s", req.Name)
	c.JSON(http.StatusCreated, response)
}

func ListCerts(c *gin.Context) {
	certs, err := listAllCerts()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to list certs: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	c.JSON(http.StatusOK, certs)
}

func DownloadCert(c *gin.Context) {
	kind := c.Query("kind")
	name := c.Query("name")
	base := c.Query("base")

	if kind == "" || name == "" || base == "" {
		fmtErr := fmt.Errorf("kind, base and name are required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	certsBase := certsBaseDir()
	path, err := filepath.Abs(filepath.Join(certsBase, kind, base, filepath.Clean(name)))

	if err != nil {
		fmtErr := fmt.Errorf("Failed to list certs: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		fmtErr := fmt.Errorf("Certificates not found: %v", err)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	c.FileAttachment(path, filepath.Base(path))
}

func DeleteCertGroup(c *gin.Context) {
	kind := c.Query("kind")
	base := c.Query("base")
	if kind == "" || base == "" {
		fmtErr := fmt.Errorf("kind and base are required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	err := deleteCertGroup(kind, base)
	if err != nil {
		fmtErr := fmt.Errorf("Error deleting cert group: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	c.Status(http.StatusNoContent)
}

func SaveRemoteCertHandler(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.RemoteCertModeEnabled() {
		util.ErrorResponse(c, http.StatusBadRequest, "api", fmt.Errorf("remote cert mode is not enabled"))
		return
	}

	kind := c.Query("kind")
	name := c.Query("name")
	base := c.Query("base")
	if kind == "" || name == "" || base == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "api", fmt.Errorf("kind, base and name are required"))
		return
	}

	remote, err := url.Parse(cfg.Certificates.RemoteCertURL)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmt.Errorf("invalid remote certificate URL: %w", err))
		return
	}

	downloadURL := remote.ResolveReference(&url.URL{Path: "/api/certs/download"})
	req, err := http.NewRequest(http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmt.Errorf("creating remote download request: %w", err))
		return
	}

	q := req.URL.Query()
	q.Set("kind", kind)
	q.Set("base", base)
	q.Set("name", name)
	req.URL.RawQuery = q.Encode()

	resp, err := util.GetRemoteHttpClient().Do(req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadGateway, "api", fmt.Errorf("requesting remote certificate: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		util.ErrorResponse(c, http.StatusBadGateway, "api", fmt.Errorf("remote certificate download failed: %s - %s", resp.Status, strings.TrimSpace(string(body))))
		return
	}

	certsBase := cfg.Certificates.CertsDir
	destDir := filepath.Join(certsBase, kind, base)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmt.Errorf("creating certificate destination dir: %w", err))
		return
	}

	safeName := filepath.Base(name)
	destPath := filepath.Join(destDir, safeName)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmt.Errorf("reading remote certificate body: %w", err))
		return
	}

	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmt.Errorf("saving certificate to local store: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"path": destPath})
}
