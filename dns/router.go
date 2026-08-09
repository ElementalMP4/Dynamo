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
	if err := c.BindJSON(&rec); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}
	if strings.TrimSpace(rec.Name) == "" || strings.TrimSpace(rec.Type) == "" {
		fmtErr := fmt.Errorf("name and type are required")
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	rec.ID = uuid.NewString()

	dnsMutationMu.Lock()
	defer dnsMutationMu.Unlock()

	records, err := dnsStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

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
	if err := c.BindJSON(&update); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	dnsMutationMu.Lock()
	defer dnsMutationMu.Unlock()

	records, err := dnsStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	found := false
	var updatedRecord DNSRecord

	for i := range records {
		if records[i].ID != id {
			continue
		}
		found = true

		if update.Name != nil {
			records[i].Name = *update.Name
		}
		if update.Type != nil {
			records[i].Type = *update.Type
		}
		if update.TTL != nil {
			records[i].TTL = *update.TTL
		}
		if update.IPv4 != nil {
			records[i].IPv4 = *update.IPv4
		}
		if update.CNAME != nil {
			records[i].CNAME = *update.CNAME
		}
		if update.TXT != nil {
			records[i].TXT = *update.TXT
		}
		if update.Target != nil {
			records[i].Target = *update.Target
		}
		if update.Port != nil {
			records[i].Port = *update.Port
		}
		if update.Priority != nil {
			records[i].Priority = *update.Priority
		}
		if update.Weight != nil {
			records[i].Weight = *update.Weight
		}

		updatedRecord = records[i]
		break
	}

	if !found {
		fmtErr := fmt.Errorf("Record %s not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	if err := writeRecords(records); err != nil {
		fmtErr := fmt.Errorf("Failed to write DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf("dns-router", "Modified %s record %s", updatedRecord.Type, updatedRecord.Name)
	c.Status(http.StatusOK)
}

func DeleteRecord(c *gin.Context) {
	id, err := parseRecordID(c)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, err)
		return
	}

	dnsMutationMu.Lock()
	defer dnsMutationMu.Unlock()

	records, err := dnsStore.Load()

	if err != nil {
		fmtErr := fmt.Errorf("Failed to load DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	var filtered []DNSRecord
	var toBeDeleted DNSRecord
	found := false
	for _, r := range records {
		if r.ID != id {
			filtered = append(filtered, r)
		} else {
			toBeDeleted = r
			found = true
		}
	}

	if !found {
		fmtErr := fmt.Errorf("Record %s not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	if err := writeRecords(filtered); err != nil {
		fmtErr := fmt.Errorf("Failed to write DNS records: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf("dns-router", "Deleted %s record %s", toBeDeleted.Type, toBeDeleted.Name)
	c.Status(http.StatusNoContent)
}
