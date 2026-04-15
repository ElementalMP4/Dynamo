package vpn

import (
	"Dynamo/config"
	"Dynamo/store"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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

func clientConfigPath(ifaceName, peerID string) string {
	base := filepath.Join(config.GetConfig().DataDirectory, "wireguard", "clients", ifaceName)
	_ = os.MkdirAll(base, 0755)
	return filepath.Join(base, peerID+".conf")
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
PostUp = iptables -t nat -A POSTROUTING -o %s -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -o %s -j MASQUERADE
`, iface.OutInterface, iface.OutInterface)
	}

	for _, p := range iface.Peers {
		if !p.Enabled {
			continue
		}

		cfg += fmt.Sprintf(`
[Peer]
PublicKey = %s
AllowedIPs = %s
`, p.PublicKey, p.IP)
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
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`,
		peer.PrivateKey,
		peer.IP,
		dns,
		iface.PublicKey,
		iface.Endpoint,
		allowed,
	)
}

func writeServerConfig(name, cfg string) error {
	err := os.WriteFile(fmt.Sprintf("/etc/wireguard/%s.conf", name), []byte(cfg), 0600)
	if err != nil {
		return fmt.Errorf("failed to write server config: %v", err)
	}
	return nil
}

func writeClientConfig(iface Interface, peer Peer) error {
	path := clientConfigPath(iface.Name, peer.ID)
	cfg := buildClientConfig(iface, peer)
	err := os.WriteFile(path, []byte(cfg), 0600)
	if err != nil {
		return fmt.Errorf("failed to write client config for peer %s: %v", peer.Name, err)
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

			for _, p := range iface.Peers {
				if err := writeClientConfig(iface, p); err != nil {
					return err
				}
			}

			return nil
		}
		return errors.New("interface not found")
	})
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

			peer := Peer{
				ID:         uuid.New().String(),
				Name:       req.Name,
				PublicKey:  pub,
				PrivateKey: priv,
				IP:         ip,
				Enabled:    true,
				DNS:        req.DNS,
				AllowedIPs: req.AllowedIPs,
			}

			iface.Peers = append(iface.Peers, peer)
			state.Interfaces[i] = iface

			if err := writeServerConfig(iface.Name, buildServerConfig(iface)); err != nil {
				return err
			}
			if err := syncInterface(iface.Name); err != nil {
				return err
			}

			if err := writeClientConfig(iface, peer); err != nil {
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

					if err := writeClientConfig(iface, iface.Peers[j]); err != nil {
						return err
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
				} else {
					_ = os.Remove(clientConfigPath(ifaceName, peerID))
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
	state, err := vpnStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load VPN store: %v", err)
	}

	for _, iface := range state.Interfaces {
		cfg := buildServerConfig(iface)
		if err := writeServerConfig(iface.Name, cfg); err != nil {
			return fmt.Errorf("failed to writw server config: %v", err)
		}

		if err := SetInterfaceEnabled(iface.Name, iface.Enabled); err != nil {
			return fmt.Errorf("failed to toggle interface: %v", err)
		}

		for _, peer := range iface.Peers {
			if err := writeClientConfig(iface, peer); err != nil {
				return fmt.Errorf("failed to write client %s config: %v", peer.Name, err)
			}
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

				return syncInterface(iface.Name)
			} else {
				if err := exec.Command("systemctl", "stop", svc).Run(); err != nil {
					return fmt.Errorf("failed to stop interface: %v", err)
				}

				if err := exec.Command("systemctl", "disable", svc).Run(); err != nil {
					return fmt.Errorf("failed to disable interface: %v", err)
				}
			}

			return nil
		}

		return errors.New("interface not found")
	})
}
