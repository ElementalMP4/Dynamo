package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"TinyVTM/config"
	"TinyVTM/log"

	"github.com/gin-gonic/gin"
)

// GET /api/nginx/list
// Returns list of available configs and which are enabled
func listNginxConfigs(c *gin.Context) {
	cfg := config.GetConfig()
	availDir := cfg.GetNginxSitesAvailable()
	enabledDir := cfg.GetNginxSitesEnabled()

	// Read available configs
	entries, err := os.ReadDir(availDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read sites-available: %v", err)})
		return
	}

	type ConfigInfo struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	configs := []ConfigInfo{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".conf") {
			continue
		}

		// Check if enabled (symlinked)
		enabledPath := filepath.Join(enabledDir, name)
		enabled := false
		if fi, err := os.Lstat(enabledPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			enabled = true
		}

		configs = append(configs, ConfigInfo{
			Name:    name,
			Enabled: enabled,
		})
	}

	c.JSON(http.StatusOK, configs)
}

// GET /api/nginx/read/:name
// Read a config file from sites-available
func readNginxConfig(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
		name += ".conf"
	}

	cfg := config.GetConfig()
	path := filepath.Join(cfg.GetNginxSitesAvailable(), name)

	// Prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(path), cfg.GetNginxSitesAvailable()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config name"})
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": string(content)})
}

// PUT /api/nginx/save/:name
// Save a config file to sites-available
func saveNginxConfig(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
		name += ".conf"
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := config.GetConfig()
	path := filepath.Join(cfg.GetNginxSitesAvailable(), name)

	// Prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(path), cfg.GetNginxSitesAvailable()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config name"})
		return
	}

	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save config: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /api/nginx/enable/:name
// Create symlink in sites-enabled
func enableNginxConfig(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
		name += ".conf"
	}

	cfg := config.GetConfig()
	availPath := filepath.Join(cfg.GetNginxSitesAvailable(), name)
	enablePath := filepath.Join(cfg.GetNginxSitesEnabled(), name)

	// Check source exists
	if _, err := os.Stat(availPath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to check config: %v", err)})
		return
	}

	// Remove existing symlink if any
	_ = os.Remove(enablePath)

	// Create new symlink
	if err := os.Symlink(availPath, enablePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to enable config: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}

// DELETE /api/nginx/disable/:name
// Remove symlink from sites-enabled
func disableNginxConfig(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
		name += ".conf"
	}

	cfg := config.GetConfig()
	path := filepath.Join(cfg.GetNginxSitesEnabled(), name)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to disable config: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}

// DELETE /api/nginx/delete/:name
// Delete config from sites-available (and sites-enabled if enabled)
func deleteNginxConfig(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
		name += ".conf"
	}

	cfg := config.GetConfig()
	availPath := filepath.Join(cfg.GetNginxSitesAvailable(), name)
	enablePath := filepath.Join(cfg.GetNginxSitesEnabled(), name)

	// Remove from sites-enabled first if it exists
	_ = os.Remove(enablePath)

	// Remove from sites-available
	if err := os.Remove(availPath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete config: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /api/nginx/reload
// Reload nginx via systemctl
func reloadNginx(c *gin.Context) {
	cfg := config.GetConfig()
	service := cfg.GetNginxServiceName()

	cmd := exec.Command("systemctl", "reload", service)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.LogError(fmt.Sprintf("Failed to reload nginx: %v\n%s", err, out))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to reload nginx: %v", err)})
		return
	}

	c.Status(http.StatusNoContent)
}
