package dns

import (
	"Dynamo/types"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const dnsmasqFile = "/etc/dnsmasq.d/dynamo.conf"

func EnsureWritable() error {
	f, err := os.OpenFile(dnsmasqFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	f.Close()

	return nil
}

func reloadDnsmasq() error {
	cmd := exec.Command("systemctl", "restart", "dnsmasq")
	return cmd.Run()
}

func loadRecords() ([]types.DNSRecord, error) {
	file, err := os.Open(dnsmasqFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []types.DNSRecord
	scanner := bufio.NewScanner(file)
	id := 1
	ttl := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# ttl=") {
			ttl, _ = strconv.Atoi(strings.TrimPrefix(line, "# ttl="))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rec := types.DNSRecord{
			ID:    id,
			TTL:   ttl,
			Class: "IN",
		}

		switch {
		case strings.HasPrefix(line, "address=/"):
			rec.Type = "A"
			parts := strings.Split(strings.TrimPrefix(line, "address=/"), "/")
			rec.Name = parts[0]
			rec.IPv4 = parts[1]

		case strings.HasPrefix(line, "cname="):
			rec.Type = "CNAME"
			parts := strings.Split(strings.TrimPrefix(line, "cname="), ",")
			rec.Name = parts[0]
			rec.CNAME = parts[1]

		case strings.HasPrefix(line, "txt-record="):
			rec.Type = "TXT"
			parts := strings.SplitN(strings.TrimPrefix(line, "txt-record="), ",", 2)
			rec.Name = parts[0]
			rec.TXT = []string{strings.Trim(parts[1], `"`)}

		case strings.HasPrefix(line, "srv-host="):
			rec.Type = "SRV"
			parts := strings.Split(strings.TrimPrefix(line, "srv-host="), ",")
			rec.Name = parts[0]
			rec.Target = parts[1]
			port, _ := strconv.Atoi(parts[2])
			rec.Port = port
			priority, _ := strconv.Atoi(parts[3])
			rec.Priority = priority
			weight, _ := strconv.Atoi(parts[4])
			rec.Weight = weight
		default:
			continue
		}

		records = append(records, rec)
		id++
	}

	return records, nil
}

func writeRecords(records []types.DNSRecord) error {
	var out strings.Builder

	out.WriteString("# Managed by Dynamo\n\n")

	for _, r := range records {
		out.WriteString(fmt.Sprintf("# ttl=%d\n", r.TTL))

		switch r.Type {
		case "A":
			out.WriteString(fmt.Sprintf("address=/%s/%s\n", r.Name, r.IPv4))
		case "CNAME":
			out.WriteString(fmt.Sprintf("cname=%s,%s\n", r.Name, r.CNAME))
		case "TXT":
			for _, t := range r.TXT {
				out.WriteString(fmt.Sprintf("txt-record=%s,\"%s\"\n", r.Name, t))
			}
		case "SRV":
			out.WriteString(fmt.Sprintf(
				"srv-host=%s,%s,%d,%d,%d\n",
				r.Name, r.Target, r.Port, r.Priority, r.Weight,
			))
		}

		out.WriteString("\n")
	}

	if err := os.WriteFile(dnsmasqFile, []byte(out.String()), 0644); err != nil {
		return err
	}

	return reloadDnsmasq()
}

func ListRecords(c *gin.Context) {
	records, err := loadRecords()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, records)
}

func CreateRecord(c *gin.Context) {
	var rec types.DNSRecord
	if err := c.BindJSON(&rec); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	records, _ := loadRecords()
	rec.ID = len(records) + 1
	records = append(records, rec)

	if err := writeRecords(records); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(201)
}

func GetRecordByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	records, _ := loadRecords()

	for _, r := range records {
		if r.ID == id {
			c.JSON(200, r)
			return
		}
	}

	c.JSON(404, gin.H{"error": "not found"})
}

func UpdateRecord(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var update types.DNSRecord
	if err := c.BindJSON(&update); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	records, err := loadRecords()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	for i := range records {
		if records[i].ID == id {
			records[i] = update

			if err := writeRecords(records); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.Status(200)
			return
		}
	}

	c.JSON(404, gin.H{"error": "not found"})
}

func DeleteRecord(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	records, _ := loadRecords()

	var filtered []types.DNSRecord
	for _, r := range records {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}

	if err := writeRecords(filtered); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}
