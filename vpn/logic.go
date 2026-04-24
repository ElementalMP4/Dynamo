package vpn

import (
	"Dynamo/store"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/google/uuid"
)

const loggingSource string = "vpn-router"

var vpnStore = store.NewJSONStore[State]("wireguard.json")

var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var validIface = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func generateKeyPair() (string, string, error) {
	privOut, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	privateKey := strings.TrimSpace(string(privOut))

	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(privateKey)

	pubOut, err := pubCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %v", err)
	}

	return privateKey, strings.TrimSpace(string(pubOut)), nil
}

func nextIP(subnet string, peers []Peer) (string, error) {
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("failed to parse subnet: %v", err)
	}

	used := map[string]bool{}
	for _, p := range peers {
		used[p.IP] = true
	}

	base := ip.To4()

	for i := 2; i < 255; i++ {
		candidate := net.IPv4(base[0], base[1], base[2], byte(i)).String() + "/32"
		if !used[candidate] {
			return candidate, nil
		}
	}

	return "", errors.New("no available IPs")
}

func buildServerConfig(iface Interface) string {
	cfg := fmt.Sprintf(`[Interface]
Address = %s
ListenPort = %d
PrivateKey = %s
SaveConfig = true
`, iface.Address, iface.ListenPort, iface.PrivateKey)

	if iface.EnableNAT {
		cfg += fmt.Sprintf(`
PostUp = iptables -A FORWARD -i %s -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE; ip6tables -A FORWARD -i %s -j ACCEPT; ip6tables -t nat -A POSTROUTING -o %s -j MASQUERADE
PostDown = iptables -D FORWARD -i %s -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE; ip6tables -D FORWARD -i %s -j ACCEPT; ip6tables -t nat -D POSTROUTING -o %s -j MASQUERADE
`, iface.Name, iface.OutInterface, iface.Name, iface.OutInterface, iface.Name, iface.OutInterface, iface.Name, iface.OutInterface)
	}

	for _, p := range iface.Peers {
		if !p.Enabled {
			log.Infof("vpn", "Omitting peer %s", p.Name)
			continue
		}

		log.Infof("vpn", "Including peer %s", p.Name)

		cfg += fmt.Sprintf(`[Peer]
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s
`, p.PublicKey, p.PresharedKey, p.IP)
	}

	return cfg
}

func buildClientConfig(iface Interface, peer Peer) string {
	dns := iface.DefaultDNS
	if peer.DNS != nil {
		dns = *peer.DNS
	}

	allowed := iface.DefaultAllowedIPs
	if peer.AllowedIPs != nil {
		allowed = *peer.AllowedIPs
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
PresharedKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`,
		peer.PrivateKey,
		peer.IP,
		dns,
		iface.PublicKey,
		peer.PresharedKey,
		iface.Endpoint,
		allowed,
	)
}

func writeServerConfig(name, cfg string) error {
	// Ensure interface name is a single path component and cannot escape safeDir.
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") ||
		strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("invalid interface name")
	}

	path := filepath.Join("/etc/wireguard", name+".conf")
	err := os.WriteFile(path, []byte(cfg), 0600)
	if err != nil {
		return fmt.Errorf("failed to write server config: %v", err)
	}
	return nil
}

func syncInterface(name string) error {
	out, err := exec.Command("wg-quick", "strip", name).Output()
	if err != nil {
		return fmt.Errorf("failed to strip interface %s: %v", name, err)
	}

	cmd := exec.Command("wg", "syncconf", name, "/dev/stdin")
	cmd.Stdin = strings.NewReader(string(out))

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to sync interface config %s: %v", name, err)
	}
	return nil
}

func enableAndStart(name string) error {
	svc := "wg-quick@" + name
	if err := exec.Command("systemctl", "enable", svc).Run(); err != nil {
		return fmt.Errorf("failed to enable interface %s: %v", name, err)
	}
	if err := exec.Command("systemctl", "start", svc).Run(); err != nil {
		return fmt.Errorf("failed to start interface %s: %v", name, err)
	}

	return nil
}

func CreateInterface(req CreateInterfaceRequest) (*Interface, error) {
	if !validName.MatchString(req.Name) {
		return nil, errors.New("invalid interface name")
	}

	if req.EnableNAT && (!validIface.MatchString(req.OutInterface)) {
		return nil, errors.New("invalid out_interface")
	}

	if req.DefaultDNS == "" {
		req.DefaultDNS = "1.1.1.1"
	}
	if req.DefaultAllowedIPs == "" {
		req.DefaultAllowedIPs = "0.0.0.0/0"
	}

	var created *Interface

	err := vpnStore.Update(func(state *State) error {
		for _, i := range state.Interfaces {
			if i.Name == req.Name {
				return errors.New("interface already exists")
			}
		}

		priv, pub, err := generateKeyPair()
		if err != nil {
			return err
		}

		iface := Interface{
			Name:              req.Name,
			Address:           req.Address,
			ListenPort:        req.ListenPort,
			PublicKey:         pub,
			PrivateKey:        priv,
			EnableNAT:         req.EnableNAT,
			OutInterface:      req.OutInterface,
			DefaultDNS:        req.DefaultDNS,
			DefaultAllowedIPs: req.DefaultAllowedIPs,
			Endpoint:          req.Endpoint,
			Peers:             []Peer{},
		}

		cfg := buildServerConfig(iface)

		if err := writeServerConfig(iface.Name, cfg); err != nil {
			return err
		}

		if err := enableAndStart(iface.Name); err != nil {
			return err
		}

		state.Interfaces = append(state.Interfaces, iface)
		created = &iface
		return nil
	})

	return created, err
}

func UpdateInterface(name string, req UpdateInterfaceRequest) error {
	return vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != name {
				continue
			}

			if req.DefaultDNS != nil {
				iface.DefaultDNS = *req.DefaultDNS
			}
			if req.DefaultAllowedIPs != nil {
				iface.DefaultAllowedIPs = *req.DefaultAllowedIPs
			}
			if req.Endpoint != nil {
				iface.Endpoint = *req.Endpoint
			}
			if req.EnableNAT != nil {
				iface.EnableNAT = *req.EnableNAT
			}
			if req.OutInterface != nil {
				iface.OutInterface = *req.OutInterface
			}

			state.Interfaces[i] = iface

			cfg := buildServerConfig(iface)
			if err := writeServerConfig(iface.Name, cfg); err != nil {
				return err
			}

			if err := syncInterface(iface.Name); err != nil {
				return err
			}

			return nil
		}
		return errors.New("interface not found")
	})
}

func generatePresharedKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func AddPeer(ifaceName string, req CreatePeerRequest) (*Peer, error) {
	var created *Peer

	err := vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != ifaceName {
				continue
			}

			ip, err := nextIP(iface.Address, iface.Peers)
			if err != nil {
				return err
			}

			priv, pub, err := generateKeyPair()
			if err != nil {
				return err
			}

			psk, err := generatePresharedKey()
			if err != nil {
				return err
			}

			peer := Peer{
				ID:           uuid.New().String(),
				Name:         req.Name,
				PublicKey:    pub,
				PrivateKey:   priv,
				IP:           ip,
				Enabled:      true,
				DNS:          req.DNS,
				AllowedIPs:   req.AllowedIPs,
				PresharedKey: psk,
			}

			iface.Peers = append(iface.Peers, peer)
			state.Interfaces[i] = iface

			if err := writeServerConfig(iface.Name, buildServerConfig(iface)); err != nil {
				return err
			}
			if err := syncInterface(iface.Name); err != nil {
				return err
			}

			created = &peer
			return nil
		}
		return errors.New("interface not found")
	})

	return created, err
}

func UpdatePeer(ifaceName, peerID string, req UpdatePeerRequest) error {
	return vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != ifaceName {
				continue
			}

			for j := range iface.Peers {
				if iface.Peers[j].ID == peerID {
					if req.Name != nil {
						iface.Peers[j].Name = *req.Name
					}
					if req.Enabled != nil {
						iface.Peers[j].Enabled = *req.Enabled
					}
					if req.DNS != nil {
						iface.Peers[j].DNS = req.DNS
					}
					if req.AllowedIPs != nil {
						iface.Peers[j].AllowedIPs = req.AllowedIPs
					}
				}
			}

			state.Interfaces[i] = iface

			if err := writeServerConfig(iface.Name, buildServerConfig(iface)); err != nil {
				return err
			}
			return syncInterface(iface.Name)
		}
		return errors.New("interface not found")
	})
}

func DeletePeer(ifaceName, peerID string) error {
	return vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != ifaceName {
				continue
			}

			var peers []Peer
			for _, p := range iface.Peers {
				if p.ID != peerID {
					peers = append(peers, p)
				}
			}

			iface.Peers = peers
			state.Interfaces[i] = iface

			if err := writeServerConfig(iface.Name, buildServerConfig(iface)); err != nil {
				return err
			}
			return syncInterface(iface.Name)
		}
		return errors.New("interface not found")
	})
}

func SyncWireguard() error {
	log.Info("vpn-sync", "Syncing VPN")
	state, err := vpnStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load VPN store: %v", err)
	}

	for _, iface := range state.Interfaces {
		log.Infof("vpn-sync", "Syncing interface %s", iface.Name)
		if err := SetInterfaceEnabled(iface.Name, iface.Enabled); err != nil {
			return fmt.Errorf("failed to toggle interface: %v", err)
		}
	}

	return nil
}

func SetInterfaceEnabled(name string, enabled bool) error {
	return vpnStore.Update(func(state *State) error {
		for i, iface := range state.Interfaces {
			if iface.Name != name {
				continue
			}

			iface.Enabled = enabled
			state.Interfaces[i] = iface
			svc := "wg-quick@" + iface.Name

			if enabled {
				log.Infof("vpn", "Enabling interface %s", iface.Name)

				cfg := buildServerConfig(iface)
				if err := writeServerConfig(iface.Name, cfg); err != nil {
					return fmt.Errorf("failed to write server config: %v", err)
				}

				if err := exec.Command("systemctl", "enable", svc).Run(); err != nil {
					return fmt.Errorf("failed to enable interface: %v", err)
				}
				if err := exec.Command("systemctl", "start", svc).Run(); err != nil {
					return fmt.Errorf("failed to start interface: %v", err)
				}

				if err := syncInterface(iface.Name); err != nil {
					return err
				}

				log.Okf("vpn", "Enabled interface %s", iface.Name)
			} else {
				log.Infof("vpn", "Disabling interface %s", iface.Name)
				if err := exec.Command("systemctl", "stop", svc).Run(); err != nil {
					return fmt.Errorf("failed to stop interface: %v", err)
				}

				if err := exec.Command("systemctl", "disable", svc).Run(); err != nil {
					return fmt.Errorf("failed to disable interface: %v", err)
				}
				log.Okf("vpn", "Disabled interface %s", iface.Name)
			}

			return nil
		}

		return errors.New("interface not found")
	})
}

func getWireguardDump() ([][]string, error) {
	out, err := exec.Command("wg", "show", "all", "dump").Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	var result [][]string
	for _, line := range lines {
		result = append(result, strings.Split(line, "\t"))
	}

	return result, nil
}

func collectMetrics() ([]InterfaceStats, error) {
	lines, err := getWireguardDump()
	if err != nil {
		return nil, err
	}

	state, _ := vpnStore.Load()

	peerKeyMap := map[string]string{}
	peerEnabledMap := map[string]bool{}
	for _, iface := range state.Interfaces {
		for _, p := range iface.Peers {
			peerKeyMap[p.PublicKey] = p.ID
			peerEnabledMap[p.PublicKey] = p.Enabled
		}
	}

	result := map[string]*InterfaceStats{}

	for _, fields := range lines {
		// Skip the interface line
		if len(fields) < 8 {
			continue
		}

		ifaceName := fields[0]
		if !state.ContainsInterface(ifaceName) {
			continue
		}

		//0: interface 1: public-key 2: preshared-key 3: endpoint 4: allowed-ips 5: latest-handshake 6: transfer-rx 7: transfer-tx 8: persistent-keepalive

		pubKey := fields[1]
		lastHandshake, _ := strconv.ParseInt(fields[5], 10, 64)
		rxBytes, _ := strconv.ParseUint(fields[6], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[7], 10, 64)

		if _, ok := result[ifaceName]; !ok {
			result[ifaceName] = &InterfaceStats{
				Name: ifaceName,
			}
		}

		now := time.Now().Unix()
		active := false
		if lastHandshake > 0 && (now-lastHandshake) <= inactiveTimeout {
			active = true
		}

		peerID := peerKeyMap[pubKey]
		enabled := peerEnabledMap[pubKey]
		ps := PeerStats{
			ID:            peerID,
			RX:            rxBytes,
			TX:            txBytes,
			LastHandshake: lastHandshake,
			Active:        active,
			Enabled:       enabled,
		}

		result[ifaceName].Peers = append(result[ifaceName].Peers, ps)
		result[ifaceName].RX += rxBytes
		result[ifaceName].TX += txBytes
	}

	var out []InterfaceStats
	for _, v := range result {
		out = append(out, *v)
	}

	return out, nil
}
