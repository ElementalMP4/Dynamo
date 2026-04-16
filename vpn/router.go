package vpn

import (
	"Dynamo/config"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
)

func CreateInterfaceHandler(c *gin.Context) {
	var req CreateInterfaceRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	res, err := CreateInterface(req)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

func ListInterfacesHandler(c *gin.Context) {
	state, _ := vpnStore.Load()
	c.JSON(200, state.Interfaces)
}

func UpdateInterfaceHandler(c *gin.Context) {
	var req UpdateInterfaceRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := UpdateInterface(c.Param("name"), req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}

func CreatePeerHandler(c *gin.Context) {
	var req CreatePeerRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	peer, err := AddPeer(c.Param("name"), req)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, peer)
}

func ListPeersHandler(c *gin.Context) {
	state, _ := vpnStore.Load()
	for _, i := range state.Interfaces {
		if i.Name == c.Param("name") {
			c.JSON(200, i.Peers)
			return
		}
	}
	c.JSON(404, gin.H{"error": "not found"})
}

func UpdatePeerHandler(c *gin.Context) {
	var req UpdatePeerRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := UpdatePeer(c.Param("name"), c.Param("peer"), req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}

func DeletePeerHandler(c *gin.Context) {
	if err := DeletePeer(c.Param("name"), c.Param("peer")); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}

func GetPeerConfigHandler(c *gin.Context) {
	state, _ := vpnStore.Load()
	for _, iface := range state.Interfaces {
		if iface.Name != c.Param("name") {
			continue
		}
		for _, p := range iface.Peers {
			if p.ID == c.Param("peer") {
				c.String(200, buildClientConfig(iface, p))
				return
			}
		}
	}
	c.JSON(404, gin.H{"error": "not found"})
}

var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func isSafeInterfaceName(name string) bool {
	return interfaceNamePattern.MatchString(name)
}

func DeleteInterfaceHandler(c *gin.Context) {
	name := c.Param("name")
	if !isSafeInterfaceName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interface name"})
		return
	}

	err := vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != name {
				continue
			}

			svc := "wg-quick@" + name
			_ = exec.Command("systemctl", "stop", svc).Run()
			_ = exec.Command("systemctl", "disable", svc).Run()

			_ = os.Remove(fmt.Sprintf("/etc/wireguard/%s.conf", name))

			clientDir := filepath.Join(
				config.GetConfig().DataDirectory,
				"wireguard",
				"clients",
				name,
			)
			_ = os.RemoveAll(clientDir)
			state.Interfaces = append(state.Interfaces[:i], state.Interfaces[i+1:]...)
			return nil
		}

		return errors.New("interface not found")
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func ToggleInterfaceHandler(c *gin.Context) {
	name := c.Param("name")

	var req ToggleInterfaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("Couldn't unpack JSON: %v", err)})
		return
	}

	if err := SetInterfaceEnabled(name, req.Enabled); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("Couldn't enable VPN interface: %v", err)})
		return
	}

	c.Status(204)
}

func MetricsHandler(c *gin.Context) {
	stats, err := collectMetrics()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"interfaces": stats,
	})
}
