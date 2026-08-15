package api

import (
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"Dynamo/util"
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

func getSystemInfoHandler(c *gin.Context) {
	cfg := config.GetConfig()

	if cfg.RemoteCertModeEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"remoteCertMode": cfg.RemoteCertModeEnabled(),
			"remoteCertURL":  cfg.Certificates.RemoteCertURL,
			"remoteCertMTLS": cfg.RemoteCertMtlsEnabled(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"remoteCertMode": cfg.RemoteCertModeEnabled(),
		})
	}
}

func getCountsHandler(c *gin.Context) {
	cfg := config.GetConfig()
	dnsCounts := dns.CountDNSRecords()

	var certCounts map[string]int
	var err error
	if cfg.RemoteCertModeEnabled() {
		certCounts, err = fetchRemoteCertCounts(cfg)
		if err != nil {
			errFmt := fmt.Errorf("failed to fetch remote cert counts: %v", err)
			log.Errorf("stats", "%v", errFmt)
			c.JSON(500, gin.H{"error": errFmt})
			return
		}
	} else {
		certCounts, err = certificate.CountCerts()
		if err != nil {
			errFmt := fmt.Errorf("failed to count certs: %v", err.Error())
			log.Errorf("stats", "%v", errFmt)
			c.JSON(500, gin.H{"error": errFmt})
			return
		}
	}

	circuitCounts, err := circuit.CountCircuits()
	if err != nil {
		errFmt := fmt.Errorf("failed to count circuits: %v", err.Error())
		log.Errorf("stats", "%v", errFmt)
		c.JSON(500, gin.H{"error": errFmt})
		return
	}

	c.JSON(200, gin.H{"dns": dnsCounts, "certs": certCounts, "circuits": circuitCounts, "instance": cfg.System.InstanceName})
}

func getNetStats() (map[string]InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ipMap, err := buildInterfaceIPMap()
	if err != nil {
		// Not fatal - we can still return throughput stats without IPs
		ipMap = map[string][]string{}
	}

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
			RX:  rx,
			TX:  tx,
			IPs: ipMap[iface],
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func buildInterfaceIPMap() (map[string][]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ipMap := make(map[string][]string, len(ifaces))

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		ips := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil {
				ips = append(ips, ip.String())
			}
		}

		ipMap[iface.Name] = ips
	}

	return ipMap, nil
}

func listNetworkInterfaces(c *gin.Context) {
	ifaces, err := listInterfaces()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to list network interfaces: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, "api", fmtErr)
		return
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

func getSystemStatsHandler(c *gin.Context) {
	cpu, err := getCPUStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	mem, err := getMemStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	net, err := getNetStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, SystemStats{CPU: cpu, Mem: mem, Net: net})
}

func getCPUStats() (CPUStats, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return CPUStats{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			break
		}
		parse := func(i int) uint64 {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			return v
		}
		return CPUStats{
			User:    parse(1),
			Nice:    parse(2),
			System:  parse(3),
			Idle:    parse(4),
			Iowait:  parse(5),
			Irq:     parse(6),
			Softirq: parse(7),
			Steal:   parse(8),
		}, nil
	}
	return CPUStats{}, fmt.Errorf("cpu stats not found in /proc/stat")
}

func getMemStats() (MemStats, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemStats{}, err
	}
	defer file.Close()

	var stats MemStats
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			stats.TotalKB = v
		case "MemAvailable:":
			stats.AvailableKB = v
		}
	}
	return stats, nil
}
