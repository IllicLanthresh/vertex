package interfaces

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

// VirtualDevice represents a virtual network device
type VirtualDevice struct {
	Name       string
	Interface  string
	IP         string
	MAC        string
	LinkIndex  int
	RouteTable int // policy routing table ID (0 = none)
}

// Manager manages network interfaces and virtual devices
type Manager struct {
	mu             sync.RWMutex
	physicalIfaces []string
	virtualDevices map[string][]*VirtualDevice
	macCounter     int
}

// NewManager creates a new interface manager
func NewManager() *Manager {
	return &Manager{
		virtualDevices: make(map[string][]*VirtualDevice),
	}
}

// DiscoverPhysicalInterfaces discovers all physical network interfaces
func (m *Manager) DiscoverPhysicalInterfaces() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	var physical []string
	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if iface.Flags&net.FlagLoopback != 0 ||
			strings.HasPrefix(iface.Name, "docker") ||
			strings.HasPrefix(iface.Name, "veth") ||
			strings.HasPrefix(iface.Name, "br-") ||
			strings.HasPrefix(iface.Name, "macvlan") ||
			strings.HasPrefix(iface.Name, "virbr") {
			continue
		}

		// Check if interface has a MAC address (indicating physical interface)
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			physical = append(physical, iface.Name)
		}
	}

	m.physicalIfaces = physical
	log.Printf("Discovered %d physical interfaces: %v", len(physical), physical)
	return nil
}

// GetPhysicalInterfaces returns the list of discovered physical interfaces
func (m *Manager) GetPhysicalInterfaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.physicalIfaces))
	copy(result, m.physicalIfaces)
	return result
}

func (m *Manager) CreateVirtualDevices(interfaceName string, count int, macPrefix string, dhcpRetries int, dhcpRetryDelay int) ([]*VirtualDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up any existing virtual devices for this interface
	if existing, ok := m.virtualDevices[interfaceName]; ok {
		for _, dev := range existing {
			m.cleanupVirtualDevice(dev)
		}
	}

	var devices []*VirtualDevice

	for i := 0; i < count; i++ {
		device, err := m.createSingleVirtualDevice(interfaceName, i, macPrefix, dhcpRetries, dhcpRetryDelay)
		if err != nil {
			log.Printf("Failed to create virtual device %d for %s: %v", i, interfaceName, err)
			continue
		}
		devices = append(devices, device)
	}

	m.virtualDevices[interfaceName] = devices
	log.Printf("Created %d virtual devices for interface %s", len(devices), interfaceName)
	return devices, nil
}

// createSingleVirtualDevice creates a single virtual device
func (m *Manager) createSingleVirtualDevice(interfaceName string, index int, macPrefix string, dhcpRetries int, dhcpRetryDelay int) (*VirtualDevice, error) {
	m.macCounter++

	// Generate device name and MAC address
	deviceName := fmt.Sprintf("macvlan%s%d", interfaceName, index)
	macAddr := m.generateMACAddress(macPrefix, m.macCounter)

	// Get parent interface
	parentLink, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent interface %s: %w", interfaceName, err)
	}

	// Create macvlan interface
	macvlan := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        deviceName,
			ParentIndex: parentLink.Attrs().Index,
		},
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}

	// Set MAC address
	mac, err := net.ParseMAC(macAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address %s: %w", macAddr, err)
	}
	macvlan.LinkAttrs.HardwareAddr = mac

	// Add the interface
	if err := netlink.LinkAdd(macvlan); err != nil {
		return nil, fmt.Errorf("failed to create macvlan interface: %w", err)
	}

	// Bring the interface up
	if err := netlink.LinkSetUp(macvlan); err != nil {
		netlink.LinkDel(macvlan) // Clean up on error
		return nil, fmt.Errorf("failed to bring up interface: %w", err)
	}

	device := &VirtualDevice{
		Name:      deviceName,
		Interface: interfaceName,
		MAC:       macAddr,
		LinkIndex: macvlan.Attrs().Index,
	}

	// Try to get IP address via DHCP
	if ip, err := m.getDHCPAddress(deviceName, dhcpRetries, dhcpRetryDelay); err == nil {
		device.IP = ip
		if err := m.setupPolicyRouting(device, interfaceName, index); err != nil {
			log.Printf("Policy routing setup failed for %s: %v (traffic may not route)", deviceName, err)
		}
	} else {
		log.Printf("Failed to get DHCP address for %s: %v", deviceName, err)
	}

	return device, nil
}

const policyRouteTableBase = 100

func (m *Manager) setupPolicyRouting(device *VirtualDevice, parentIface string, index int) error {
	gateway, err := m.getDefaultGateway(parentIface)
	if err != nil {
		return fmt.Errorf("no default gateway found for %s: %w", parentIface, err)
	}

	deviceIP := net.ParseIP(device.IP)
	if deviceIP == nil {
		return fmt.Errorf("invalid device IP: %s", device.IP)
	}

	link, err := netlink.LinkByName(device.Name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", device.Name, err)
	}

	tableID := policyRouteTableBase + index

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Gw:        gateway,
		Table:     tableID,
	}
	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to add route in table %d: %w", tableID, err)
	}

	h, err := netlink.NewHandle()
	if err != nil {
		netlink.RouteDel(route)
		return fmt.Errorf("failed to create netlink handle: %w", err)
	}
	defer h.Delete()

	rule := netlink.NewRule()
	rule.Src = &net.IPNet{IP: deviceIP, Mask: net.CIDRMask(32, 32)}
	rule.Table = tableID
	if err := h.RuleAdd(rule); err != nil {
		netlink.RouteDel(route)
		return fmt.Errorf("failed to add rule for %s: %w", device.IP, err)
	}

	device.RouteTable = tableID
	log.Printf("Policy routing: %s (%s) -> table %d via %s", device.Name, device.IP, tableID, gateway)
	return nil
}

func (m *Manager) getDefaultGateway(parentIface string) (net.IP, error) {
	link, err := netlink.LinkByName(parentIface)
	if err != nil {
		return nil, err
	}

	routes, err := netlink.RouteList(link, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, r := range routes {
		if r.Dst == nil && r.Gw != nil {
			return r.Gw, nil
		}
	}

	allRoutes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}
	for _, r := range allRoutes {
		if r.Dst == nil && r.Gw != nil {
			return r.Gw, nil
		}
	}

	return nil, fmt.Errorf("no default gateway found")
}

func (m *Manager) cleanupPolicyRouting(device *VirtualDevice) {
	if device.RouteTable == 0 || device.IP == "" {
		return
	}

	deviceIP := net.ParseIP(device.IP)
	if deviceIP == nil {
		return
	}

	h, err := netlink.NewHandle()
	if err != nil {
		log.Printf("Failed to create netlink handle for cleanup: %v", err)
		return
	}
	defer h.Delete()

	rule := netlink.NewRule()
	rule.Src = &net.IPNet{IP: deviceIP, Mask: net.CIDRMask(32, 32)}
	rule.Table = device.RouteTable
	if err := h.RuleDel(rule); err != nil {
		log.Printf("Failed to remove routing rule for %s: %v", device.IP, err)
	}
}

// generateMACAddress generates a MAC address with the given prefix
func (m *Manager) generateMACAddress(prefix string, counter int) string {
	// Remove colons from prefix and ensure it's valid
	cleanPrefix := strings.ReplaceAll(prefix, ":", "")
	if len(cleanPrefix) != 6 {
		cleanPrefix = "020000" // fallback to default
	}

	// Generate last 3 bytes from counter
	return fmt.Sprintf("%s:%02x:%02x:%02x",
		strings.Join([]string{cleanPrefix[:2], cleanPrefix[2:4], cleanPrefix[4:6]}, ":"),
		(counter>>16)&0xff,
		(counter>>8)&0xff,
		counter&0xff)
}

func (m *Manager) getDHCPAddress(interfaceName string, retries int, retryDelay int) (string, error) {
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			log.Printf("DHCP retry %d/%d for %s", attempt, retries, interfaceName)
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}

		cmd := exec.Command("dhclient", "-1", interfaceName)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("udhcpc", "-i", interfaceName, "-n", "-q")
			if err := cmd.Run(); err != nil {
				if attempt < retries {
					continue
				}
				return "", fmt.Errorf("DHCP failed after %d attempts", retries+1)
			}
		}

		link, err := netlink.LinkByName(interfaceName)
		if err != nil {
			continue
		}

		addrs, err := netlink.AddrList(link, 2) // AF_INET
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if addr.IP != nil && !addr.IP.IsLoopback() {
				return addr.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no IP address assigned after %d attempts", retries+1)
}

// GetVirtualDevices returns virtual devices for an interface
func (m *Manager) GetVirtualDevices(interfaceName string) []*VirtualDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.virtualDevices[interfaceName]
	if !ok {
		return nil
	}

	result := make([]*VirtualDevice, len(devices))
	copy(result, devices)
	return result
}

// GetAllVirtualDevices returns all virtual devices across all interfaces
func (m *Manager) GetAllVirtualDevices() map[string][]*VirtualDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*VirtualDevice)
	for iface, devices := range m.virtualDevices {
		devicesCopy := make([]*VirtualDevice, len(devices))
		copy(devicesCopy, devices)
		result[iface] = devicesCopy
	}
	return result
}

func (m *Manager) cleanupVirtualDevice(device *VirtualDevice) {
	m.cleanupPolicyRouting(device)

	link, err := netlink.LinkByName(device.Name)
	if err != nil {
		log.Printf("Failed to get link for cleanup %s: %v", device.Name, err)
		return
	}

	if err := netlink.LinkDel(link); err != nil {
		log.Printf("Failed to delete virtual device %s: %v", device.Name, err)
	}
}

// Cleanup removes all virtual devices
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for iface, devices := range m.virtualDevices {
		for _, device := range devices {
			m.cleanupVirtualDevice(device)
		}
		log.Printf("Cleaned up %d virtual devices for interface %s", len(devices), iface)
	}

	m.virtualDevices = make(map[string][]*VirtualDevice)
}

// GetInterfaceStats returns network statistics for an interface
func (m *Manager) GetInterfaceStats(interfaceName string) (*InterfaceStats, error) {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", interfaceName, err)
	}

	attrs := link.Attrs()
	stats := attrs.Statistics

	// Get IP addresses
	addrs, err := netlink.AddrList(link, 0) // AF_UNSPEC = 0 (all families)
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses: %w", err)
	}

	var ipAddrs []string
	for _, addr := range addrs {
		if addr.IP != nil {
			ipAddrs = append(ipAddrs, addr.IP.String())
		}
	}

	return &InterfaceStats{
		Name:        interfaceName,
		IsUp:        attrs.Flags&net.FlagUp != 0,
		MAC:         attrs.HardwareAddr.String(),
		IPs:         ipAddrs,
		BytesSent:   stats.TxBytes,
		BytesRecv:   stats.RxBytes,
		PacketsSent: stats.TxPackets,
		PacketsRecv: stats.RxPackets,
	}, nil
}

// InterfaceStats represents network interface statistics
type InterfaceStats struct {
	Name        string   `json:"name"`
	IsUp        bool     `json:"is_up"`
	MAC         string   `json:"mac_address"`
	IPs         []string `json:"addresses"`
	BytesSent   uint64   `json:"bytes_sent"`
	BytesRecv   uint64   `json:"bytes_recv"`
	PacketsSent uint64   `json:"packets_sent"`
	PacketsRecv uint64   `json:"packets_recv"`
}
