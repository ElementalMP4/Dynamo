package api

import (
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"Dynamo/vpn"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

func logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		info := log.RequestInfo{
			Method:  c.Request.Method,
			Path:    c.Request.URL.RequestURI(),
			Status:  c.Writer.Status(),
			IP:      c.ClientIP(),
			Latency: time.Since(start),
		}
		log.LoggingMiddleware(info)
	}
}

func apiNoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		c.Next()
	}
}

func StartAPI() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(logger())
	r.Use(apiNoStore())
	r.Use(gin.Recovery())
	htmlFS := getHTMLFileSystem()

	// Static assets
	r.StaticFS("/css", getCSSFileSystem())
	r.StaticFS("/js", getJSFileSystem())

	// Serve index.html at /  — http.FileServer on a directory serves its
	// index.html directly without issuing any redirect.
	r.GET("/", func(c *gin.Context) {
		http.FileServer(htmlFS).ServeHTTP(c.Writer, c.Request)
	})

	// Cert management routes
	cfg := config.GetConfig()
	certs := r.Group("/api/certs")

	// Alway run /save locally as this endpoint pulls certs in from the remote
	certs.POST("/save", certificate.SaveRemoteCertHandler)

	if cfg.RemoteCertModeEnabled() {
		remote, err := url.Parse(cfg.Certificates.RemoteCertURL)
		if err != nil {
			log.Fatalf("api/certs", "Invalid remote certificate URL: %v", err)
		}

		proxy := &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(remote)
				r.Out.URL.Path = r.In.URL.Path
				r.Out.URL.RawQuery = r.In.URL.RawQuery
			},
		}

		if cfg.RemoteCertMtlsEnabled() {
			tlsCfg, err := cfg.BuildMTLSConfig()
			if err != nil {
				log.Fatalf("api/certs", "Failed to build mTLS config for cert proxy: %v", err)
			}

			proxy.Transport = &http.Transport{
				TLSClientConfig: tlsCfg,
			}

			log.Info("api/certs", "Remote cert proxy using mTLS")
		}

		r.NoRoute(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/api/certs/") && !strings.HasPrefix(c.Request.URL.Path, "/api/certs") {
				c.Next()
				return
			}

			c.Request.URL.Path = strings.TrimSuffix(
				c.Request.URL.Path,
				"/",
			)

			proxy.ServeHTTP(c.Writer, c.Request)
		})
	} else {
		certs.GET("", certificate.ListCerts)
		certs.GET("/files", certificate.GetCertFiles)
		certs.DELETE("", certificate.DeleteCertGroup)
		certs.POST("/root", certificate.CreateRootCA)
		certs.POST("/server", certificate.CreateServerCert)
		certs.POST("/client", certificate.CreateClientCert)
		certs.GET("/download", certificate.DownloadCert)
	}

	// DNS routes
	api := r.Group("/api/dns")
	api.GET("", dns.ListRecords)
	api.POST("", dns.CreateRecord)
	api.GET(":id", dns.GetRecordByID)
	api.PUT(":id", dns.UpdateRecord)
	api.DELETE(":id", dns.DeleteRecord)

	// Circuit routes
	circuits := r.Group("/api/circuits")
	circuits.GET("", circuit.ListCircuits)
	circuits.POST("", circuit.CreateCircuit)
	circuits.GET(":id", circuit.GetByID)
	circuits.PUT(":id", circuit.UpdateCircuit)
	circuits.DELETE(":id", circuit.DeleteCircuit)
	circuits.POST(":id/enable", circuit.EnableCircuit)
	circuits.POST(":id/disable", circuit.DisableCircuit)

	// Wireguard routes
	wg := r.Group("/api/vpn")
	wg.POST("/interfaces", vpn.CreateInterfaceHandler)
	wg.GET("/interfaces", vpn.ListInterfacesHandler)

	wg.PUT("/interfaces/:name", vpn.UpdateInterfaceHandler)
	wg.DELETE("/interfaces/:name", vpn.DeleteInterfaceHandler)
	wg.PUT("/interfaces/:name/toggle", vpn.ToggleInterfaceHandler)

	wg.POST("/interfaces/:name/peers", vpn.CreatePeerHandler)
	wg.GET("/interfaces/:name/peers", vpn.ListPeersHandler)

	wg.PUT("/interfaces/:name/peers/:peer", vpn.UpdatePeerHandler)
	wg.DELETE("/interfaces/:name/peers/:peer", vpn.DeletePeerHandler)
	wg.GET("/interfaces/:name/peers/:peer/config", vpn.GetPeerConfigHandler)

	wg.GET("/metrics", vpn.MetricsHandler)

	// Statistics routes
	r.GET("/api/stats/dynamo", getCountsHandler)
	r.GET("/api/stats/system", getSystemStatsHandler)
	r.GET("/api/system/interfaces", listNetworkInterfaces)
	r.GET("/api/system/info", getSystemInfoHandler)

	log.Info("api", "Starting admin API on "+config.GetConfig().System.ServerBind)
	if err := r.Run(config.GetConfig().System.ServerBind); err != nil {
		log.Fatalf("api", "admin API failed: %v", err)
	}
}
