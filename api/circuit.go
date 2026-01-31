package api

import (
	"Dynamo/db"
	"Dynamo/types"
	"fmt"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func iptablesExists(args []string) bool {
	checkArgs := append([]string{"-C"}, args[1:]...)
	cmd := exec.Command("iptables", checkArgs...)
	return cmd.Run() == nil
}

func iptablesAdd(args []string) error {
	if iptablesExists(args) {
		return nil
	}
	cmd := exec.Command("iptables", args...)
	return cmd.Run()
}

func iptablesDelete(args []string) error {
	delArgs := append([]string{"-D"}, args[1:]...)
	cmd := exec.Command("iptables", delArgs...)
	return cmd.Run()
}

func EnableCircuit(c types.RoutingCircuit) error {
	for _, rule := range c.IPTablesRules() {
		if err := iptablesAdd(rule); err != nil {
			return err
		}
	}
	return nil
}

func DisableCircuit(c types.RoutingCircuit) error {
	for _, rule := range c.IPTablesRules() {
		_ = iptablesDelete(rule) // best-effort
	}
	return nil
}

func SyncCircuits() error {
	rows, err := db.DB().Query(`SELECT * FROM routing_circuits`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		rc, err := db.ScanCircuitRows(rows)
		if err != nil {
			return err
		}

		if rc.Enabled {
			if err := EnableCircuit(rc); err != nil {
				return fmt.Errorf("enable %s failed: %w", rc.Name, err)
			}
		} else {
			DisableCircuit(rc)
		}
	}

	return rows.Err()
}

func createCircuit(c *gin.Context) {
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
}

func enableCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(
		`SELECT * FROM routing_circuits WHERE id = ?`,
		id,
	)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if err := EnableCircuit(rc); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	_, err = db.DB().Exec(
		`UPDATE routing_circuits SET enabled = 1 WHERE id = ?`,
		id,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func disableCircuit(c *gin.Context) {
	id := c.Param("id")

	row := db.DB().QueryRow(
		`SELECT * FROM routing_circuits WHERE id = ?`,
		id,
	)

	rc, err := db.ScanCircuitRow(row)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	DisableCircuit(rc)

	_, err = db.DB().Exec(
		`UPDATE routing_circuits SET enabled = 0 WHERE id = ?`,
		id,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func listCircuits(c *gin.Context) {
	rows, err := db.DB().Query(`SELECT * FROM routing_circuits ORDER BY id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var circuits []types.RoutingCircuit
	for rows.Next() {
		rc, err := db.ScanCircuitRows(rows)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		circuits = append(circuits, rc)
	}

	c.JSON(200, circuits)
}

func updateCircuit(c *gin.Context) {
	id := c.Param("id")

	var existing types.RoutingCircuit
	row := db.DB().QueryRow(`SELECT * FROM routing_circuits WHERE id = ?`, id)
	if err := row.Scan( /* same fields as scanCircuit */ ); err != nil {
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

	_, err := db.DB().Exec(`
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
	)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func deleteCircuit(c *gin.Context) {
	id := c.Param("id")

	var rc types.RoutingCircuit
	row := db.DB().QueryRow(`SELECT * FROM routing_circuits WHERE id = ?`, id)
	if err := row.Scan( /* fields */ ); err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if rc.Enabled {
		DisableCircuit(rc)
	}

	_, err := db.DB().Exec(`DELETE FROM routing_circuits WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}
