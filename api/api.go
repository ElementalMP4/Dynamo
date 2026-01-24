package api

import (
	"Dynamo/certificate"
	"Dynamo/config"
	"Dynamo/log"
	"Dynamo/types"
	"net/http"

	"github.com/gin-gonic/gin"
)

func StartAPI() {
	err := certificate.EnsureSslDirs()
	if err != nil {
		panic(err) //TODO
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
	api.GET("/name/:name", getRecordByName)
	api.PUT(":id", updateRecord)
	api.DELETE(":id", deleteRecord)

	log.LogInfo("Starting admin API on " + config.GetConfig().ServerAddress)
	if err := r.Run(config.GetConfig().ServerAddress); err != nil {
		log.LogError("admin API failed: " + err.Error())
	}
}

func listRecords(c *gin.Context) {

}

func createRecord(c *gin.Context) {
	var rec types.DnsRecord
	if err := c.ShouldBindJSON(&rec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rec.Name == "" || rec.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing name or type"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{})
}

func getRecordByID(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func getRecordByName(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}
func updateRecord(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func deleteRecord(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}
