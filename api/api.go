package api

import (
	"Dynamo/certificate"
	"Dynamo/config"
	"Dynamo/log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const dnsmasqFile = "/etc/dnsmasq.d/dynamo.conf"

func StartAPI() {
	err := certificate.EnsureSslDirs()
	if err != nil {
		panic(err)
	}

	err = ensureWritable()
	if err != nil {
		panic(err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Load embedded HTML templates
	templ, err := loadEmbeddedTemplates()
	if err != nil {
		log.LogFatal("Failed to load templates: " + err.Error())
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
	api.GET("", listRecords)
	api.POST("", createRecord)
	api.GET("/id/:id", getRecordByID)
	api.PUT(":id", updateRecord)
	api.DELETE(":id", deleteRecord)

	circuits := r.Group("/api/circuits")
	circuits.GET("", listCircuits)
	circuits.POST("", createCircuit)
	circuits.PUT(":id", updateCircuit)
	circuits.POST(":id/enable", enableCircuit)
	circuits.POST(":id/disable", disableCircuit)
	circuits.DELETE(":id", deleteCircuit)

	log.LogInfo("Starting admin API on " + config.GetConfig().ServerAddress)
	if err := r.Run(config.GetConfig().ServerAddress); err != nil {
		log.LogError("admin API failed: " + err.Error())
	}
}
