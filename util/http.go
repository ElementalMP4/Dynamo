package util

import (
	"Dynamo/config"
	"net/http"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

var remoteHttpClient *http.Client

func init() {
	remoteHttpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	cfg := config.GetConfig()
	if cfg.RemoteCertMtlsEnabled() {
		tlsCfg, err := cfg.BuildMTLSConfig()
		if err != nil {
			log.Fatalf("api", "failed to build remote TLS transport: %v", err)
		}
		remoteHttpClient.Transport = &http.Transport{TLSClientConfig: tlsCfg}
	}
}

func GetRemoteHttpClient() *http.Client {
	return remoteHttpClient
}

func ErrorResponse(c *gin.Context, httpStatus int, source string, err error) {
	c.JSON(httpStatus, gin.H{"error": err.Error()})
	log.Errorf(source, "HTTP Error: %v", err)
}
