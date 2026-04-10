package dns

import (
	"Dynamo/util"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

func ListRecords(c *gin.Context) {
	records, err := loadRecords()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}
	c.JSON(http.StatusOK, records)
}

func CreateRecord(c *gin.Context) {
	var rec DNSRecord
	if err := c.BindJSON(&rec); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	records, _ := loadRecords()
	rec.ID = len(records) + 1
	records = append(records, rec)

	if err := writeRecords(records); err != nil {
		fmtErr := fmt.Errorf("Failed to write DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Created %s record %s", rec.Type, rec.Name)
	c.Status(http.StatusCreated)
}

func GetRecordByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	records, _ := loadRecords()

	for _, r := range records {
		if r.ID == id {
			c.JSON(http.StatusOK, r)
			return
		}
	}
	fmtErr := fmt.Errorf("DNS record %d not found", id)
	util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
}

func UpdateRecord(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var update DNSRecord
	if err := c.BindJSON(&update); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	records, err := loadRecords()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	for i := range records {
		if records[i].ID != id {
			continue
		}

		if update.Name != "" && update.Name != records[i].Name {
			records[i].Name = update.Name
		}
		if update.Type != "" && update.Type != records[i].Type {
			records[i].Type = update.Type
		}
		if update.TTL != records[i].TTL {
			records[i].TTL = update.TTL
		}
		if update.IPv4 != "" && update.IPv4 != records[i].IPv4 {
			records[i].IPv4 = update.IPv4
		}
		if update.CNAME != "" && update.CNAME != records[i].CNAME {
			records[i].CNAME = update.CNAME
		}
		if len(update.TXT) > 0 {
			records[i].TXT = update.TXT
		}
		if update.Target != "" && update.Target != records[i].Target {
			records[i].Target = update.Target
		}
		if update.Port != records[i].Port && update.Port != 0 {
			records[i].Port = update.Port
		}
		if update.Priority != records[i].Priority && update.Priority != 0 {
			records[i].Priority = update.Priority
		}
		if update.Weight != records[i].Weight && update.Weight != 0 {
			records[i].Weight = update.Weight
		}

		if err := writeRecords(records); err != nil {
			fmtErr := fmt.Errorf("Failed to write DNS records: %v", err)
			util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
			return
		}

		log.Okf("dns-router", "Modified %s record %s", update.Type, update.Name)
		c.Status(http.StatusOK)
		return

	}

	fmtErr := fmt.Errorf("Record %d not found", id)
	util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
}

func DeleteRecord(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	records, _ := loadRecords()

	var filtered []DNSRecord
	var toBeDeleted DNSRecord
	for _, r := range records {
		if r.ID != id {
			filtered = append(filtered, r)
		} else {
			toBeDeleted = r
		}
	}

	if err := writeRecords(filtered); err != nil {
		fmtErr := fmt.Errorf("Failed to write DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf("dns-router", "Deleted %s record %s", toBeDeleted.Type, toBeDeleted.Name)
	c.Status(http.StatusNoContent)
}
