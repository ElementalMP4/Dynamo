package api

import (
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"Dynamo/util"
	"Dynamo/vpn"
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
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

func StartAPI() {
	err := certificate.EnsureSslDirs()
	if err != nil {
		log.Fatalf("api", "Failed to check SSL dirs: %v", err)
	}

	err = dns.EnsureWritable()
	if err != nil {
		log.Fatalf("api", "DNS config file is not writable: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(logger())
	r.Use(gin.Recovery())

	// UI routes
	templ, err := loadEmbeddedTemplates()
	if err != nil {
		log.Fatalf("api", "Failed to load templates: %v", err.Error())
	}
	r.SetHTMLTemplate(templ)
	r.StaticFS("/static", getFileSystem())

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"isHome": true,
		})
	})

	r.GET("/dns", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"isDns": true,
		})
	})

	r.GET("/certs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"isCerts": true,
		})
	})

	r.GET("/circuits", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"isCircuits": true,
		})
	})

	r.GET("/vpn", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"isVpn": true,
		})
	})

	// Cert management routes
	certs := r.Group("/api/certs")
	certs.GET("", certificate.ListCerts)
	certs.DELETE("", certificate.DeleteCertGroup)
	certs.POST("/root", certificate.CreateRootCA)
	certs.POST("/server", certificate.CreateServerCert)
	certs.POST("/client", certificate.CreateClientCert)
	certs.GET("/download", certificate.DownloadCert)

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

	wg.POST("/interfaces/:name/peers", vpn.CreatePeerHandler)
	wg.GET("/interfaces/:name/peers", vpn.ListPeersHandler)

	wg.PUT("/interfaces/:name/peers/:peer", vpn.UpdatePeerHandler)
	wg.DELETE("/interfaces/:name/peers/:peer", vpn.DeletePeerHandler)
	wg.GET("/interfaces/:name/peers/:peer/config", vpn.GetPeerConfigHandler)

	// Statistics routes
	r.GET("/api/stats/dynamo", getCountsHandler)
	r.GET("/api/stats/network", getNetStatsHandler)
	r.GET("/api/system/interfaces", listNetworkInterfaces)

	log.Info("api", "Starting admin API on "+config.GetConfig().ServerAddress)
	if err := r.Run(config.GetConfig().ServerAddress); err != nil {
		log.Fatalf("api", "admin API failed: %v", err)
	}
}

func getCountsHandler(c *gin.Context) {
	dnsCounts, err := dns.CountDNSRecords()

	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to count DNS records: %v", err.Error())})
		return
	}

	certCounts, err := certificate.CountCerts()
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to count certs: %v", err.Error())})
		return
	}

	circuitCounts, err := circuit.CountCircuits()
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to count circuits: %v", err.Error())})
		return
	}

	c.JSON(200, gin.H{"dns": dnsCounts, "certs": certCounts, "circuits": circuitCounts})
}

func getNetStatsHandler(c *gin.Context) {
	stats, err := getNetStats()
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, stats)
}

func getNetStats() (map[string]InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := map[string]InterfaceStats{}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.Split(line, ":")

		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])

		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)

		stats[iface] = InterfaceStats{
			RX: rx,
			TX: tx,
		}
	}

	return stats, nil
}

func listNetworkInterfaces(c *gin.Context) {
	ifaces, err := listInterfaces()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to list network interfaces: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmtErr)
	}

	c.JSON(http.StatusOK, gin.H{"interfaces": ifaces})
}

func listInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %v", err)
	}

	var out []string
	for _, i := range ifaces {
		out = append(out, i.Name)
	}

	return out, nil
}
