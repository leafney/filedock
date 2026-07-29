package identityx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	RoleWorker   = "worker"
	RoleFollower = "follower"
)

type Identity struct {
	ID          string            `json:"id"`
	ShortID     string            `json:"short_id"`
	Role        string            `json:"role"`
	Hostname    string            `json:"hostname"`
	Platform    string            `json:"platform"`
	MACAddrs    []string          `json:"mac_addresses"`
	MachineID   string            `json:"machine_id"`
	GeneratedAt time.Time         `json:"generated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Profile struct {
	Role      string
	Hostname  string
	Platform  string
	MACAddrs  []string
	MachineID string
	Metadata  map[string]string
}

func Generate(role string) (*Identity, error) {
	profile := Profile{
		Role:      role,
		Hostname:  hostname(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		MACAddrs:  collectMACAddresses(),
		MachineID: readMachineID(),
		Metadata:  collectMetadata(),
	}
	return FromProfile(profile, time.Now())
}

func FromProfile(profile Profile, now time.Time) (*Identity, error) {
	prefix, err := rolePrefix(profile.Role)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.Hostname) == "" {
		profile.Hostname = "unknown"
	}
	if strings.TrimSpace(profile.Platform) == "" {
		profile.Platform = "unknown"
	}
	if profile.Metadata == nil {
		profile.Metadata = map[string]string{}
	}

	id := buildStableID(prefix, profile)
	shortID := id
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	return &Identity{
		ID:          id,
		ShortID:     shortID,
		Role:        profile.Role,
		Hostname:    profile.Hostname,
		Platform:    profile.Platform,
		MACAddrs:    append([]string(nil), profile.MACAddrs...),
		MachineID:   profile.MachineID,
		GeneratedAt: now,
		Metadata:    cloneMap(profile.Metadata),
	}, nil
}

func IsCompatible(cached, current *Identity) bool {
	if cached == nil || current == nil {
		return false
	}
	if cached.Role != "" && current.Role != "" && cached.Role != current.Role {
		return false
	}
	if cached.MachineID != "" && current.MachineID != "" && cached.MachineID != current.MachineID {
		return false
	}
	if cached.MachineID != "" && current.MachineID != "" {
		return true
	}
	if len(cached.MACAddrs) > 0 && len(current.MACAddrs) > 0 {
		for _, oldMAC := range cached.MACAddrs {
			for _, newMAC := range current.MACAddrs {
				if oldMAC == newMAC {
					return true
				}
			}
		}
		return false
	}
	return true
}

func buildStableID(prefix string, profile Profile) string {
	parts := []string{
		strings.TrimSpace(profile.Hostname),
		strings.TrimSpace(profile.Platform),
		strings.TrimSpace(profile.MachineID),
		strings.Join(profile.MACAddrs, ","),
		strings.TrimSpace(profile.Metadata["container_id"]),
		strings.TrimSpace(profile.Metadata["k8s_namespace"]),
		strings.TrimSpace(profile.Metadata["k8s_pod_name"]),
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])

	host := sanitizeHostname(profile.Hostname)
	if host == "" {
		host = "unknown"
	}
	if len(host) > 8 {
		host = host[:8]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, host, hash[:16])
}

func rolePrefix(role string) (string, error) {
	switch role {
	case RoleWorker:
		return "w", nil
	case RoleFollower:
		return "f", nil
	default:
		return "", fmt.Errorf("unsupported identity role %q", role)
	}
}

func sanitizeHostname(hostname string) string {
	out := strings.ToLower(strings.TrimSpace(hostname))
	out = strings.ReplaceAll(out, ".", "-")
	out = strings.ReplaceAll(out, "_", "-")
	out = strings.Trim(out, "-")
	return out
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "unknown"
	}
	return name
}

func collectMACAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var macs []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		if isVirtualInterface(name) {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			macs = append(macs, iface.HardwareAddr.String())
		}
	}
	return macs
}

func isVirtualInterface(name string) bool {
	prefixes := []string{"docker", "veth", "virbr", "br-", "lo"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func readMachineID() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	for _, file := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(file)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func collectMetadata() map[string]string {
	metadata := map[string]string{}
	if value := strings.TrimSpace(os.Getenv("POD_NAME")); value != "" {
		metadata["k8s_pod_name"] = value
	}
	if value := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); value != "" {
		metadata["k8s_namespace"] = value
	}
	if value := readContainerID(); value != "" {
		metadata["container_id"] = value
	}
	return metadata
}

func readContainerID() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "docker") && !strings.Contains(line, "containerd") {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) == 0 {
			continue
		}
		last := strings.TrimSpace(parts[len(parts)-1])
		if len(last) >= 12 {
			return last[:12]
		}
	}
	return ""
}

func cloneMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
