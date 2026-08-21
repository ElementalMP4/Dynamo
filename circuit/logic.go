package circuit

import (
	"fmt"

	"github.com/ElementalMP4/loggerino/log"

	"Dynamo/store"
)

const loggingSource string = "circuit-router"

var circuitStore *store.JSONStore[[]RoutingCircuit]

func nextID(circuits []RoutingCircuit) int64 {
	var max int64
	for _, c := range circuits {
		if c.ID > max {
			max = c.ID
		}
	}
	return max + 1
}

func findCircuitByID(circuits []RoutingCircuit, id int64) (*RoutingCircuit, int) {
	for i, c := range circuits {
		if c.ID == id {
			return &c, i
		}
	}
	return nil, -1
}

func SyncCircuits() error {
	circuitStore = store.NewJSONStore[[]RoutingCircuit]("circuits.json")
	log.Info("circuit-sync", "Syncing circuits...")

	circuits, err := circuitStore.Load()
	if err != nil {
		return err
	}

	for _, rc := range circuits {
		if rc.Enabled {
			if err := rc.EnableCircuit(); err != nil {
				return fmt.Errorf("enable %s failed: %w", rc.Name, err)
			}
		} else {
			if err := rc.DisableCircuit(); err != nil {
				return fmt.Errorf("disable %s failed: %w", rc.Name, err)
			}
		}
	}

	log.Okf("circuit-sync", "Circuit sync completed successfully!")

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
