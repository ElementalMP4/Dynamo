package circuit

import (
	"Dynamo/db"
	"Dynamo/types"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const iptablesTimeout = 10 * time.Second

func expandProtocols(c types.RoutingCircuit) []types.RoutingCircuit {
	if c.Protocol != "both" {
		return []types.RoutingCircuit{c}
	}

	tcp := c
	tcp.Protocol = "tcp"

	udp := c
	udp.Protocol = "udp"

	return []types.RoutingCircuit{tcp, udp}
}

func iptablesExists(args []string) bool {
	if len(args) < 2 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), iptablesTimeout)
	defer cancel()

	checkArgs := append([]string{"-C"}, args[1:]...)
	cmd := exec.CommandContext(ctx, "iptables", checkArgs...)
	return cmd.Run() == nil
}

func iptablesAdd(args []string) error {
	if iptablesExists(args) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), iptablesTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "iptables", args...)
	return cmd.Run()
}

func iptablesDelete(args []string) error {
	if len(args) < 2 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), iptablesTimeout)
	defer cancel()

	delArgs := append([]string{"-D"}, args[1:]...)
	cmd := exec.CommandContext(ctx, "iptables", delArgs...)
	return cmd.Run()
}

func enableCircuit(c types.RoutingCircuit) error {
	log.Printf("Enabling circuit %s", c.Name)
	for _, expanded := range expandProtocols(c) {
		for _, rule := range expanded.IPTablesRules() {
			if err := iptablesAdd(rule); err != nil {
				return err
			}
		}
	}
	log.Printf("Enabled circuit %s successfully", c.Name)
	return nil
}

func dropCircuitConntrack(c types.RoutingCircuit) error {
	var errs []error

	protos := []string{c.Protocol}
	if c.Protocol == "both" {
		protos = []string{"tcp", "udp"}
	}

	for _, proto := range protos {
		cmd := exec.Command(
			"conntrack",
			"-D",
			"-p", proto,
			"--dport", strconv.Itoa(c.SrcPort),
		)

		if err := cmd.Run(); err != nil {
			// conntrack returns exit 1 if no flows matched, so we can ignore this
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				continue
			}
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("conntrack errors: %v", errs)
	}

	return nil
}

func deleteCircuitRulesByComment(c types.RoutingCircuit) error {
	comment := fmt.Sprintf("dynamo-circuit-%d", c.ID)
	// lists all rules, removes the ones with the dynamo comment, then loads them all back in
	deleteCommand := fmt.Sprintf("iptables-save | grep -v '%s' | iptables-restore", comment)
	cmd := exec.Command("sh", "-c", deleteCommand)

	return cmd.Run()
}

func disableCircuit(c types.RoutingCircuit) error {
	log.Printf("Disabling circuit %s", c.Name)
	if err := deleteCircuitRulesByComment(c); err != nil {
		return fmt.Errorf("failed to remove firewall rules: %w", err)
	}

	if err := dropCircuitConntrack(c); err != nil {
		return fmt.Errorf("rules removed but failed to drop connections: %w", err)
	}

	log.Printf("Disabled circuit %s successfully", c.Name)
	return nil
}

func SyncCircuits() error {
	log.Println("Syncing circuits")
	rows, err := db.DB().Query(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		rc, err := db.ScanCircuitRows(rows)
		if err != nil {
			return err
		}

		log.Printf("Syncing circuit %s", rc.Name)
		if rc.Enabled {
			if err := enableCircuit(rc); err != nil {
				return fmt.Errorf("enable %s failed: %w", rc.Name, err)
			}
		} else {
			if err := disableCircuit(rc); err != nil {
				return fmt.Errorf("disable %s failed: %w", rc.Name, err)
			}
		}
		log.Printf("Sync circuit %s completed successfully", rc.Name)
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

func CreateCircuit(c *gin.Context) {
	var rc types.RoutingCircuit
	if err := c.ShouldBindJSON(&rc); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := db.DB().Exec(`
		INSERT INTO routing_circuits (
			name,
			type,
			enabled,
			in_interface,
			protocol,
			src_port,
			redirect_port,
			dest_ip,
			dest_port
		) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?)
	`,
		rc.Name,
		rc.Type,
		rc.InInterface,
		rc.Protocol,
		rc.SrcPort,
		rc.RedirectPort,
		rc.DestIP,
		rc.DestPort,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rc.ID = id
	rc.Enabled = false
	c.JSON(201, rc)
	log.Printf("Created circuit %s", rc.Name)
}

func EnableCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits WHERE id = ?
	`, id)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	tx, err := db.DB().Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if err := enableCircuit(rc); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(`UPDATE routing_circuits SET enabled = 1 WHERE id = ?`, id); err != nil {
		if derr := disableCircuit(rc); derr != nil {
			c.JSON(500, gin.H{
				"error": "enable failed and rollback failed: " + derr.Error(),
			})
			return
		}
		tx.Rollback()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	tx.Commit()
	c.Status(204)
}

func DisableCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits WHERE id = ?
	`, id)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if !rc.Enabled {
		c.Status(204)
		return
	}

	tx, err := db.DB().Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if err := disableCircuit(rc); err != nil {
		tx.Rollback()
		errMsg := fmt.Errorf("Failed to disable circuit: %v", err)
		log.Println(errMsg)
		c.JSON(500, gin.H{"error": errMsg})
		return
	}

	if _, err := tx.Exec(`UPDATE routing_circuits SET enabled = 0 WHERE id = ?`, id); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	tx.Commit()
	c.Status(204)
}

func listAllCircuits() ([]types.RoutingCircuit, error) {
	rows, err := db.DB().Query(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var circuits []types.RoutingCircuit
	for rows.Next() {
		rc, err := db.ScanCircuitRows(rows)
		if err != nil {
			return nil, err
		}
		circuits = append(circuits, rc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return circuits, nil
}

func ListCircuits(c *gin.Context) {
	circuits, err := listAllCircuits()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, circuits)
}

func CountCircuits() (map[string]map[string]int, error) {
	circuits, err := listAllCircuits()
	if err != nil {
		return nil, err
	}

	out := map[string]map[string]int{
		"DNAT": {
			"on":  0,
			"off": 0,
		},
		"REDIRECT": {
			"on":  0,
			"off": 0,
		},
	}

	for _, c := range circuits {
		state := "off"
		if c.Enabled {
			state = "on"
		}

		if _, ok := out[string(c.Type)]; !ok {
			continue // Unrecognised circuit type
		}

		out[string(c.Type)][state]++
	}

	return out, nil
}

func UpdateCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits WHERE id = ?
	`, id)

	existing, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if existing.Enabled {
		c.JSON(409, gin.H{"error": "disable circuit before editing"})
		return
	}

	var updated types.RoutingCircuit
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if _, err := db.DB().Exec(`
		UPDATE routing_circuits SET
			name = ?,
			type = ?,
			in_interface = ?,
			protocol = ?,
			src_port = ?,
			redirect_port = ?,
			dest_ip = ?,
			dest_port = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		updated.Name,
		updated.Type,
		updated.InInterface,
		updated.Protocol,
		updated.SrcPort,
		updated.RedirectPort,
		updated.DestIP,
		updated.DestPort,
		id,
	); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
	log.Printf("Modified circuit %s", updated.Name)
}

func DeleteCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits WHERE id = ?
	`, id)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	tx, err := db.DB().Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if err := disableCircuit(rc); err != nil {
		tx.Rollback()
		errMsg := fmt.Errorf("failed to remove firewall state: %v", err)
		log.Println(errMsg)
		c.JSON(500, gin.H{"error": errMsg})
		return
	}

	if _, err := tx.Exec(`DELETE FROM routing_circuits WHERE id = ?`, id); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	tx.Commit()
	c.Status(204)
}

func getCircuitByID(id int) (*types.RoutingCircuit, error) {
	row := db.DB().QueryRow(`
		SELECT
			id, name, type, enabled, in_interface,
			protocol, src_port, redirect_port, dest_ip, dest_port
		FROM routing_circuits
		WHERE id = ?
	`, id)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		return nil, err
	}

	return &rc, nil
}

func GetByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	circuit, err := getCircuitByID(id)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(404, gin.H{"error": "Circuit not found"})
		return
	}

	c.JSON(200, circuit)
}

func listInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %v", err.Error())
	}

	interfaces := []string{}
	for _, i := range ifaces {
		interfaces = append(interfaces, i.Name)
	}

	return interfaces, nil
}

func ListNetworkInterfaces(c *gin.Context) {
	ifaces, err := listInterfaces()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"interfaces": ifaces})
}
