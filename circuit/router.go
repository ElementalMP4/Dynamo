package circuit

import (
	"Dynamo/types"
	"Dynamo/util"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

func CreateCircuit(c *gin.Context) {
	var rc types.RoutingCircuit
	if err := c.ShouldBindJSON(&rc); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	err := circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		rc.ID = nextID(*circuits)
		rc.Enabled = false
		*circuits = append(*circuits, rc)
		return nil
	})

	if err != nil {
		fmtErr := fmt.Errorf("Failed to update circuit store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Created circuit %s", rc.Name)
	c.JSON(http.StatusCreated, rc)
}

func ListCircuits(c *gin.Context) {
	circuits, err := circuitStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load circuits from store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	c.JSON(http.StatusOK, circuits)
}

func GetByID(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load circuits from store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	rc, _ := findCircuitByID(circuits, id)
	if rc == nil {
		fmtErr := fmt.Errorf("Circuit %d not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	c.JSON(http.StatusOK, rc)
}

func EnableCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load circuits from store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	rc, idx := findCircuitByID(circuits, id)
	if rc == nil {
		fmtErr := fmt.Errorf("Circuit %d not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	if rc.Enabled {
		c.Status(http.StatusNoContent)
		return
	}

	if err := rc.EnableCircuit(); err != nil {
		fmtErr := fmt.Errorf("Failed to enable circuit %s:", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	err = circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		(*circuits)[idx].Enabled = true
		return nil
	})

	if err != nil {
		_ = rc.DisableCircuit()
		fmtErr := fmt.Errorf("Save circuit state for %s failed - rolled back: %v", rc.Name, err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Enabled circuit %s", rc.Name)
	c.Status(http.StatusNoContent)
}

func DisableCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load circuits from store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	rc, idx := findCircuitByID(circuits, id)
	if rc == nil {
		fmtErr := fmt.Errorf("Circuit %d not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	if !rc.Enabled {
		c.Status(http.StatusNoContent)
		return
	}

	if err := rc.DisableCircuit(); err != nil {
		fmtErr := fmt.Errorf("Failed to disable circuit %s: %v", rc.Name, err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	err = circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		(*circuits)[idx].Enabled = false
		return nil
	})

	if err != nil {
		fmtErr := fmt.Errorf("Save circuit state for %s failed: %v", rc.Name, err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Disabled circuit %s", rc.Name)
	c.Status(http.StatusNoContent)
}

func UpdateCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var updated types.RoutingCircuit
	if err := c.ShouldBindJSON(&updated); err != nil {
		fmtErr := fmt.Errorf("Invalid JSON: %v", err)
		util.ErrorResponse(c, http.StatusBadRequest, loggingSource, fmtErr)
		return
	}

	errStatus := 500
	err := circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		for i, existing := range *circuits {
			if existing.ID == id {

				if existing.Enabled {
					errStatus = http.StatusConflict
					return errors.New("disable circuit before editing")
				}

				updated.ID = id
				updated.Enabled = existing.Enabled
				(*circuits)[i] = updated
				return nil
			}
		}
		errStatus = http.StatusNotFound
		return errors.New("not found")
	})

	if err != nil {
		fmtErr := fmt.Errorf("Error updating circuit: %v", err)
		util.ErrorResponse(c, errStatus, loggingSource, fmtErr)
		return
	}

	log.Okf(loggingSource, "Modified circuit %s", updated.Name)
	c.Status(http.StatusNoContent)
}

func DeleteCircuit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	circuits, err := circuitStore.Load()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to load circuits from store: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
		return
	}

	rc, _ := findCircuitByID(circuits, id)
	if rc == nil {
		fmtErr := fmt.Errorf("Circuit %d not found", id)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	if err := rc.DisableCircuit(); err != nil {
		fmtErr := fmt.Errorf("Failed to remove rules for %s: %v", rc.Name, err)
		util.ErrorResponse(c, http.StatusNotFound, loggingSource, fmtErr)
		return
	}

	circuitStore.Update(func(circuits *[]types.RoutingCircuit) error {
		for i, c := range *circuits {
			if c.ID == id {
				*circuits = append((*circuits)[:i], (*circuits)[i+1:]...)
				return nil
			}
		}
		return nil
	})

	log.Okf(loggingSource, "Circuit %s has been deleted", rc.Name)
	c.Status(http.StatusNoContent)
}

func ListNetworkInterfaces(c *gin.Context) {
	ifaces, err := listInterfaces()
	if err != nil {
		fmtErr := fmt.Errorf("Failed to list network interfaces: %v", err)
		util.ErrorResponse(c, http.StatusInternalServerError, loggingSource, fmtErr)
	}

	c.JSON(http.StatusOK, gin.H{"interfaces": ifaces})
}
