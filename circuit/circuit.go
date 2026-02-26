package circuit

import (
	"Dynamo/db"
	"Dynamo/types"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

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
		enabled, err := rc.CircuitIsEnabled()
		if err != nil {
			return err
		}

		if rc.Enabled {
			// Skip already-enabled rules
			if enabled {
				log.Printf("Circuit %s is already enabled", rc.Name)
				continue
			}

			if err := rc.EnableCircuit(); err != nil {
				return fmt.Errorf("enable %s failed: %w", rc.Name, err)
			}
		} else {
			// Skip already-disabled rules
			if !enabled {
				log.Printf("Circuit %s is already disabled", rc.Name)
				continue
			}

			if err := rc.DisableCircuit(); err != nil {
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

	if err := rc.EnableCircuit(); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(`UPDATE routing_circuits SET enabled = 1 WHERE id = ?`, id); err != nil {
		if derr := rc.DisableCircuit(); derr != nil {
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

	if err := rc.DisableCircuit(); err != nil {
		tx.Rollback()
		errMsg := fmt.Sprintf("Failed to disable circuit: %v", err)
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
			dest_port = ?
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

	if err := rc.DisableCircuit(); err != nil {
		tx.Rollback()
		errMsg := fmt.Sprintf("failed to remove firewall state: %v", err)
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
