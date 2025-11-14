package api

import (
	"TinyVTM/certificate"
	"TinyVTM/config"
	"TinyVTM/db"
	"TinyVTM/dns"
	"TinyVTM/log"
	"TinyVTM/types"
	"net/http"
	"strconv"

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
	recs, err := db.GetAllDnsRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, recs)
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
	id, err := db.CreateDnsRecord(&rec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dns.AddOrUpdateRecord(rec)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func getRecordByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	recs, err := db.GetAllDnsRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, rec := range recs {
		if rec.ID == id {
			c.JSON(http.StatusOK, rec)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func getRecordByName(c *gin.Context) {
	name := c.Param("name")
	recs, err := db.GetAllDnsRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, rec := range recs {
		if rec.Name == name {
			c.JSON(http.StatusOK, rec)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}
func updateRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Load existing record so we can merge missing fields from the incoming payload.
	recs, err := db.GetAllDnsRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var existing *types.DnsRecord
	for _, r := range recs {
		if r.ID == id {
			tmp := r
			existing = &tmp
			break
		}
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var incoming types.DnsRecord
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Merge: start with existing, overwrite with any fields provided in incoming.
	merged := *existing
	if incoming.Name != "" {
		merged.Name = incoming.Name
	}
	if incoming.Type != "" {
		merged.Type = incoming.Type
	}
	if incoming.TTL != 0 {
		merged.TTL = incoming.TTL
	}
	if incoming.Class != "" {
		merged.Class = incoming.Class
	}
	if len(incoming.IPv4) != 0 {
		merged.IPv4 = incoming.IPv4
	}
	if incoming.CNAME != "" {
		merged.CNAME = incoming.CNAME
	}
	if len(incoming.TXT) != 0 {
		merged.TXT = incoming.TXT
	}
	if incoming.Priority != 0 {
		merged.Priority = incoming.Priority
	}
	if incoming.Weight != 0 {
		merged.Weight = incoming.Weight
	}
	if incoming.Port != 0 {
		merged.Port = incoming.Port
	}
	if incoming.Target != "" {
		merged.Target = incoming.Target
	}

	merged.ID = id

	if err := db.UpdateDnsRecord(id, &merged); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dns.AddOrUpdateRecord(merged)
	c.Status(http.StatusNoContent)
}

func deleteRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	recs, err := db.GetAllDnsRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var found *types.DnsRecord
	for _, rec := range recs {
		if rec.ID == id {
			rcopy := rec
			found = &rcopy
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := db.DeleteDnsRecord(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dns.DeleteRecordByNameType(found.Name, found.Type)
	c.Status(http.StatusNoContent)
}
