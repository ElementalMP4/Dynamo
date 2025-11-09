package api

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"github.com/gin-gonic/gin"
)

type NginxConfigRequest struct {
	ServerName      string `json:"serverName"`
	ProxyPass       string `json:"proxyPass"`
	EnableWebsocket bool   `json:"enableWebsocket"`
	EnableSSL       bool   `json:"enableSsl"`
	SSLCertBase     string `json:"sslCertBase,omitempty"`
}

const nginxConfigTemplate = `{{ if .EnableSSL -}}
# HTTP redirect to HTTPS
server {
    listen 80;
    server_name {{ .ServerName }};
    return 301 https://$host$request_uri;
}

{{ end -}}
server {
    server_name {{ .ServerName }};
{{ if .EnableSSL -}}
    listen 443 ssl;

    ssl_certificate /etc/nginx/ssl/{{ .SSLCertBase }}.crt;
    ssl_certificate_key /etc/nginx/ssl/{{ .SSLCertBase }}.key;

{{ end -}}
    location / {
        proxy_pass {{ .ProxyPass }};

{{ if .EnableWebsocket -}}
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

{{ end -}}
        # Standard reverse proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`

// POST /api/nginx/config
// Generates an nginx reverse proxy configuration based on the provided parameters
func generateNginxConfig(c *gin.Context) {
	var req NginxConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.ServerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serverName is required"})
		return
	}
	if req.ProxyPass == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "proxyPass is required"})
		return
	}
	if req.EnableSSL && req.SSLCertBase == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sslCertBase is required when SSL is enabled"})
		return
	}

	// Parse and execute the template
	tmpl, err := template.New("nginx").Parse(nginxConfigTemplate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("template parse error: %v", err)})
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("template render error: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": buf.String(),
	})
}
