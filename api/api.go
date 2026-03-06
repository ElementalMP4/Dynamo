package api

import (
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"Dynamo/types"
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func StartAPI() {
	err := certificate.EnsureSslDirs()
	if err != nil {
		log.Fatalf("Failed to check SSL dirs: %v", err)
	}

	err = dns.EnsureWritable()
	if err != nil {
		log.Fatalf("DNS config file is not writable: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Load embedded HTML templates
	templ, err := loadEmbeddedTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err.Error())
	}
	r.SetHTMLTemplate(templ)

	// Serve embedded static files
	r.StaticFS("/static", getFileSystem())

	// Web routes
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

	// Cert management routes
	certs := r.Group("/api/certs")
	certs.DELETE("", certificate.DeleteCertGroup)
	certs.POST("/root", certificate.CreateRootCA)
	certs.POST("/server", certificate.CreateServerCert)
	certs.POST("/client", certificate.CreateClientCert)
	certs.GET("/list", certificate.ListCerts)
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
	circuits.GET("/interfaces", circuit.ListNetworkInterfaces)

	r.GET("/stats/dynamo", getCountsHandler)
	r.GET("/stats/network", getNetStatsHandler)

	log.Println("Starting admin API on " + config.GetConfig().ServerAddress)
	if err := r.Run(config.GetConfig().ServerAddress); err != nil {
		log.Fatalf("admin API failed: %v", err.Error())
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

func getNetStats() (map[string]types.InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := map[string]types.InterfaceStats{}

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

		stats[iface] = types.InterfaceStats{
			RX: rx,
			TX: tx,
		}
	}

	return stats, nil
}
