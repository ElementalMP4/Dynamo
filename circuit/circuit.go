package circuit

import (
	"Dynamo/types"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"

	"Dynamo/store"
)

var circuitStore *store.JSONStore[[]types.RoutingCircuit]

func nextID(circuits []types.RoutingCircuit) int64 {
	var max int64
	for _, c := range circuits {
		if c.ID > max {
			max = c.ID
		}
	}
	return max + 1
}

func findCircuitByID(circuits []types.RoutingCircuit, id int64) (*types.RoutingCircuit, int) {
	for i, c := range circuits {
		if c.ID == id {
			return &c, i
		}
	}
	return nil, -1
}

func SyncCircuits() error {
	circuitStore = store.NewJSONStore[[]types.RoutingCircuit]("circuits.json")
	log.Info("circuit-sync", "Syncing circuits")

	circuits, err := circuitStore.Load()
	if err != nil {
		return err
	}

	for _, rc := range circuits {
		log.Infof("circuit-sync", "Syncing circuit %s", rc.Name)

		if rc.Enabled {
			if err := rc.EnableCircuit(); err != nil {
				return fmt.Errorf("enable %s failed: %w", rc.Name, err)
			}
		} else {
			if err := rc.DisableCircuit(); err != nil {
				return fmt.Errorf("disable %s failed: %w", rc.Name, err)
			}
		}

		log.Okf("circuit-sync", "Sync circuit %s completed successfully", rc.Name)
	}

	return nil
}

func CreateCircuit(c *gin.Context) {
	var rc types.RoutingCircuit
	if err := c.ShouldBindJSON(&rc); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		rc.ID = nextID(*circuits)
		rc.Enabled = false
		*circuits = append(*circuits, rc)
		return nil
	})

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	log.Okf("circuit-router", "Created circuit %s", rc.Name)
	c.JSON(201, rc)
}

func ListCircuits(c *gin.Context) {
	circuits, err := circuitStore.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, circuits)
}

func GetByID(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rc, _ := findCircuitByID(circuits, id)
	if rc == nil {
		c.JSON(404, gin.H{"error": "Circuit not found"})
		return
	}

	c.JSON(200, rc)
}

func EnableCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rc, idx := findCircuitByID(circuits, id)
	if rc == nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if err := rc.EnableCircuit(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	err = circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		(*circuits)[idx].Enabled = true
		return nil
	})

	if err != nil {
		_ = rc.DisableCircuit()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func DisableCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rc, idx := findCircuitByID(circuits, id)
	if rc == nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if !rc.Enabled {
		c.Status(204)
		return
	}

	if err := rc.DisableCircuit(); err != nil {
		errMsg := fmt.Sprintf("Failed to disable circuit: %v", err)
		log.Error("circuit-router", errMsg)
		c.JSON(500, gin.H{"error": errMsg})
		return
	}

	err = circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		(*circuits)[idx].Enabled = false
		return nil
	})

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func UpdateCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var updated types.RoutingCircuit
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		for i, existing := range *circuits {
			if existing.ID == id {

				if existing.Enabled {
					return errors.New("disable circuit before editing")
				}

				updated.ID = id
				updated.Enabled = existing.Enabled
				(*circuits)[i] = updated
				return nil
			}
		}
		return errors.New("not found")
	})

	if err != nil {
		switch err.Error() {
		case "not found":
			c.JSON(404, gin.H{"error": "not found"})
		case "disable circuit before editing":
			c.JSON(409, gin.H{"error": err.Error()})
		default:
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return
	}

	log.Okf("circuit-router", "Modified circuit %s", updated.Name)
	c.Status(204)
}

func DeleteCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rc, _ := findCircuitByID(circuits, id)
	if rc == nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	if err := rc.DisableCircuit(); err != nil {
		errMsg := fmt.Sprintf("failed to remove firewall state: %v", err)
		log.Error("circuit-router", errMsg)
		c.JSON(500, gin.H{"error": errMsg})
		return
	}

	err = circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		for i, c := range *circuits {
			if c.ID == id {
				*circuits = append((*circuits)[:i], (*circuits)[i+1:]...)
				return nil
			}
		}
		return errors.New("not found")
	})

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func CountCircuits() (map[string]map[string]int, error) {
	circuits, err := circuitStore.Load()
	if err != nil {
		return nil, err
	}

	out := map[string]map[string]int{
		"DNAT":     {"on": 0, "off": 0},
		"REDIRECT": {"on": 0, "off": 0},
		"LOCAL":    {"on": 0, "off": 0},
	}

	for _, c := range circuits {
		state := "off"
		if c.Enabled {
			state = "on"
		}

		if _, ok := out[string(c.Type)]; ok {
			out[string(c.Type)][state]++
		}
	}

	return out, nil
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

func ListNetworkInterfaces(c *gin.Context) {
	ifaces, err := listInterfaces()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"interfaces": ifaces})
}
