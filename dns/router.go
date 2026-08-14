package dns

import (
	"Dynamo/util"
	"fmt"
	"net/http"
	"strings"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func parseRecordID(c *gin.Context) (string, error) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return "", fmt.Errorf("missing record id")
	}

	if _, err := uuid.Parse(id); err != nil {
		return "", fmt.Errorf("invalid record id")
	}

	return id, nil
}

func ListRecords(c *gin.Context) {
	records, err := dnsStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}
	c.JSON(http.StatusOK, records)
}

func CreateRecord(c *gin.Context) {
	var rec DNSRecord
	if err := c.ShouldBindJSON(&rec); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}
	if strings.TrimSpace(rec.Name) == "" || strings.TrimSpace(rec.Type) == "" {
		fmtErr := fmt.Errorf("name and type are required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	err := createNewRecord(rec)
	if err != nil {
		fmtErr := fmt.Errorf("failed to create DNS record: %v", err)
		log.Errorf(loggingSource, "%v", fmtErr)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Created %s record %s", rec.Type, rec.Name)
	c.Status(http.StatusCreated)
}

func GetRecordByID(c *gin.Context) {
	id, err := parseRecordID(c)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	records, err := dnsStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	for _, r := range records {
		if r.ID == id {
			c.JSON(http.StatusOK, r)
			return
		}
	}
	fmtErr := fmt.Errorf("DNS record %s not found", id)
	util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
}

func UpdateRecord(c *gin.Context) {
	id, err := parseRecordID(c)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	var update DNSRecordPatch
	if err := c.ShouldBindJSON(&update); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	updated, exists, err := patchRecord(id, update)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, err)
		return
	}

	if !exists {
		err = fmt.Errorf("record does not exist")
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, err)
		return
	}

	log.Okf("dns-router", "Modified %s record %s", updated.Type, updated.Name)
	c.Status(http.StatusOK)
}

func DeleteRecord(c *gin.Context) {
	id, err := parseRecordID(c)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	toBeDeleted, exists, err := deleteRecord(id)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, err)
		return
	}

	if !exists {
		err = fmt.Errorf("record does not exist")
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, err)
		return
	}

	log.Okf("dns-router", "Deleted %s record %s", toBeDeleted.Type, toBeDeleted.Name)
	c.Status(http.StatusNoContent)
}
