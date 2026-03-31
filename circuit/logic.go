package circuit

import (
	"Dynamo/types"
	"fmt"
	"net"

	"github.com/ElementalMP4/loggerino/log"

	"Dynamo/store"
)

const loggingSource string = "circuit-router"

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
