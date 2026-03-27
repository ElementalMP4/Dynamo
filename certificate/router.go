package certificate

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

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

	crtPath, keyPath, nginxMtls, nginxRedirect, err := createServerCert(req.Domain, req.CaBase, req.Days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Okf("cert-router", "Created server cert for %s", req.Domain)

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = "rootCA"
	}
	if req.Days == 0 {
		req.Days = 3650
	}

	crtPath, keyPath, err := createRootCA(req.Name, req.Days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Okf("cert-router", "Created root CA %s", req.Name)
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

	crtPath, keyPath, p12Path, warning, err := createClientCert(req.Name, req.Identity, req.CaBase, req.Days, req.P12Pwd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{"cert": crtPath, "key": keyPath}
	if p12Path != "" {
		response["p12"] = p12Path
	} else {
		response["p12"] = nil
	}
	if warning != "" {
		response["warning"] = warning
	}

	log.Okf("cert-router", "Created client cert %s", req.Name)
	c.JSON(http.StatusCreated, response)
}

func ListCerts(c *gin.Context) {
	certs, err := listAllCerts()
	if err != nil {
		log.Errorf("cert-router", "An error occurred whilst listing certificates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, certs)
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
		log.Errorf("cert-router", "An error occurred whilst listing certificates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list certs: %v", err)})
		return
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		log.Errorf("cert-router", "Certificate file not found: %v", err)
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

	err := deleteCertGroup(kind, base)
	if err != nil {
		log.Errorf("cert-router", "Error deleting cert group: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Error deleting cert group: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}
