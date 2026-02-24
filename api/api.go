package api

import (
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"log"
	"net/http"

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
	certs.POST("/root", certificate.CreateRootCA)
	certs.POST("/server", certificate.CreateServerCert)
	certs.POST("/client", certificate.CreateClientCert)
	certs.DELETE("", certificate.DeleteCertGroup)
	certs.GET("/list", certificate.ListCerts)
	certs.GET("/download", certificate.DownloadCert)

	// DNS routes
	api := r.Group("/api/dns")
	api.GET("", dns.ListRecords)
	api.POST("", dns.CreateRecord)
	api.GET("/id/:id", dns.GetRecordByID)
	api.PUT(":id", dns.UpdateRecord)
	api.DELETE(":id", dns.DeleteRecord)

	// Circuit routes
	circuits := r.Group("/api/circuits")
	circuits.GET("", circuit.ListCircuits)
	circuits.POST("", circuit.CreateCircuit)
	circuits.PUT(":id", circuit.UpdateCircuit)
	circuits.POST(":id/enable", circuit.EnableCircuit)
	circuits.POST(":id/disable", circuit.DisableCircuit)
	circuits.DELETE(":id", circuit.DeleteCircuit)

	log.Println("Starting admin API on " + config.GetConfig().ServerAddress)
	if err := r.Run(config.GetConfig().ServerAddress); err != nil {
		log.Fatalf("admin API failed: %v", err.Error())
	}
}
