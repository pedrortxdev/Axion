package db

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

type Network struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	Gateway   string    `json:"gateway"`
	DNS1      string    `json:"dns1"`
	VlanID    int       `json:"vlan_id"`
	IsPublic  bool      `json:"is_public"`
	CreatedAt time.Time `json:"created_at"`
}

// AllocateIP finds a free IP across available networks using a "Smart Pool" strategy.
// It supports both pre-populated (legacy) and sparse (new) allocation models.
func (s *Service) AllocateIP(ctx context.Context, instanceName string) (string, error) {
	// 1. Determine Plan Type (Placeholder for now, default to Free/Private)
	// In the future, we can check user quota/plan here.
	isPro := false

	// 2. Fetch candidate networks
	networks, err := s.getAvailableNetworks(ctx, isPro)
	if err != nil {
		return "", fmt.Errorf("failed to fetch networks: %w", err)
	}

	// 3. Try allocation in each network
	for _, net := range networks {
		ip, err := s.tryAllocateInNetwork(ctx, net, instanceName)
		if err == nil {
			log.Printf("[IPAM] Allocated %s from network %s (%s)", ip, net.Name, net.CIDR)
			return ip, nil
		}
		// Log but continue to next network
		// log.Printf("[IPAM] Pool %s full or error: %v", net.Name, err)
	}

	return "", fmt.Errorf("no IP addresses available in any pool")
}

func (s *Service) getAvailableNetworks(ctx context.Context, isPro bool) ([]Network, error) {
	query := `SELECT id, name, cidr, gateway, dns1, vlan_id, is_public FROM networks WHERE is_public = $1 ORDER BY created_at ASC`

	// Free plan gets Private (is_public=false). Pro logic handles both later.
	// For now, simple bool.

	rows, err := s.QueryContext(ctx, query, isPro)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []Network
	for rows.Next() {
		var n Network
		if err := rows.Scan(&n.ID, &n.Name, &n.CIDR, &n.Gateway, &n.DNS1, &n.VlanID, &n.IsPublic); err != nil {
			return nil, err
		}
		networks = append(networks, n)
	}
	return networks, nil
}

type NetworkStats struct {
	Network
	TotalIPs     int     `json:"total_ips"`
	UsedIPs      int     `json:"used_ips"`
	UsagePercent float64 `json:"usage_percent"`
}

func (s *Service) GetNetworksWithStats(ctx context.Context) ([]NetworkStats, error) {
	// Fetch all networks
	query := `SELECT id, name, cidr, gateway, dns1, vlan_id, is_public, created_at FROM networks ORDER BY created_at ASC`
	rows, err := s.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []NetworkStats
	for rows.Next() {
		var n NetworkStats
		if err := rows.Scan(&n.ID, &n.Name, &n.CIDR, &n.Gateway, &n.DNS1, &n.VlanID, &n.IsPublic, &n.CreatedAt); err != nil {
			return nil, err
		}

		// Calculate Total IPs
		_, ipNet, _ := net.ParseCIDR(n.CIDR)
		if ipNet != nil {
			ones, _ := ipNet.Mask.Size()
			n.TotalIPs = 1 << (32 - ones)
			if n.TotalIPs > 2 {
				n.TotalIPs -= 3 // Network, Gateway, Broadcast
			}
		}

		// Count Used IPs
		countQuery := `SELECT COUNT(*) FROM ip_leases WHERE network_id = $1 AND instance_name IS NOT NULL`
		s.QueryRowContext(ctx, countQuery, n.ID).Scan(&n.UsedIPs)

		if n.TotalIPs > 0 {
			n.UsagePercent = (float64(n.UsedIPs) / float64(n.TotalIPs)) * 100
		}

		stats = append(stats, n)
	}

	return stats, nil
}

func (s *Service) CreateNetwork(ctx context.Context, n Network) error {
	query := `INSERT INTO networks (name, cidr, gateway, is_public) VALUES ($1, $2, $3, $4)`
	_, err := s.ExecContext(ctx, query, n.Name, n.CIDR, n.Gateway, n.IsPublic)
	return err
}

func (s *Service) tryAllocateInNetwork(ctx context.Context, netDef Network, instanceName string) (string, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// 1. Lock the network definition (Concurrency Control)
	// This prevents two VMs from racing for the same "next IP" calculation
	var lockedID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM networks WHERE id = $1 FOR UPDATE", netDef.ID).Scan(&lockedID)
	if err != nil {
		return "", fmt.Errorf("failed to lock network: %w", err)
	}

	// 2. Parse CIDR
	_, ipNet, err := net.ParseCIDR(netDef.CIDR)
	if err != nil {
		return "", fmt.Errorf("invalid cidr: %w", err)
	}

	startIP := ipToInt(ipNet.IP)
	maskOnes, _ := ipNet.Mask.Size()
	totalIPs := 1 << (32 - maskOnes)

	// Exclude Network, Gateway, Broadcast
	// Typical /24: .0 (Net), .1 (Gateway), .255 (Broadcast)
	// We iterate from Start+2 to End-1 (assuming .1 is Gateway)
	// Ideally we respect netDef.Gateway to skip it specifically.

	endIP := startIP + uint32(totalIPs) - 1

	// Range to search: Start+1 to End-1
	// If Gateway is .1, we skip it.

	// 3. Fetch ALL existing leases for this network (to find holes)
	// We fetch Map[IP] -> instance_name
	rows, err := tx.QueryContext(ctx, "SELECT ip, instance_name FROM ip_leases WHERE network_id = $1", netDef.ID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	leasedIPs := make(map[uint32]string) // Map IP(int) -> InstanceName/Empty
	for rows.Next() {
		var ipStr string
		var instName sql.NullString
		if err := rows.Scan(&ipStr, &instName); err != nil {
			return "", err
		}

		parsed := net.ParseIP(ipStr)
		if parsed != nil {
			val := ipToInt(parsed)
			if instName.Valid {
				leasedIPs[val] = instName.String // Used
			} else {
				leasedIPs[val] = "" // Pre-allocated but free
			}
		}
	}

	// 4. Find the Hole
	var candidateIP uint32
	found := false

	gatewayInt := ipToInt(net.ParseIP(netDef.Gateway))

	// Simple linear scan (efficient enough for /24 and /16)
	// Start at .2 usually (Start+2 if Gateway is .1)
	for i := startIP + 1; i < endIP; i++ {
		if i == gatewayInt {
			continue // Skip gateway
		}

		// Check status
		owner, exists := leasedIPs[i]

		if !exists {
			// Case A: Sparse Mode - Row doesn't exist. It's free!
			candidateIP = i
			found = true
			break
		} else if owner == "" {
			// Case B: Pre-populated Mode - Row exists, but owner is empty. It's free!
			candidateIP = i
			found = true
			break
		}
		// Else: Owned, continue
	}

	if !found {
		return "", fmt.Errorf("network full")
	}

	candidateIPStr := intToIP(candidateIP).String()

	// 5. Claim IP
	// If it existed (empty owner), UPDATE. If not, INSERT.

	if _, exists := leasedIPs[candidateIP]; exists {
		// UPDATE
		_, err = tx.ExecContext(ctx,
			"UPDATE ip_leases SET instance_name = $1, allocated_at = $2 WHERE ip = $3 AND network_id = $4",
			instanceName, time.Now(), candidateIPStr, netDef.ID)
	} else {
		// INSERT
		_, err = tx.ExecContext(ctx,
			"INSERT INTO ip_leases (ip, instance_name, allocated_at, network_id) VALUES ($1, $2, $3, $4)",
			candidateIPStr, instanceName, time.Now(), netDef.ID)
	}

	if err != nil {
		return "", fmt.Errorf("failed to claim ip: %w", err)
	}

	return candidateIPStr, tx.Commit()
}

// ReleaseIP frees the IP assigned to an instance.
func (s *Service) ReleaseIP(ctx context.Context, instanceName string) error {
	// We just clear the ownership. We keep the row (switch to Pre-populated mode basically)
	// Or we could Delete if we want to stay Sparse.
	// For "Hybrid" stability, keeping it NULL is fine and safer for logs.
	query := `
        UPDATE ip_leases 
        SET instance_name = NULL, allocated_at = NULL 
        WHERE instance_name = $1
    `

	_, err := s.ExecContext(ctx, query, instanceName)
	if err != nil {
		return fmt.Errorf("failed to release IP for instance %s: %w", instanceName, err)
	}

	return nil
}

// GetInstanceIP retrieves the IP assigned to an instance.
func (s *Service) GetInstanceIP(ctx context.Context, instanceName string) (string, error) {
	query := `SELECT ip FROM ip_leases WHERE instance_name = $1`

	var ip string
	err := s.QueryRowContext(ctx, query, instanceName).Scan(&ip)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Not found, return empty
		}
		return "", err
	}

	return ip, nil
}

// --- Helpers ---

func ipToInt(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func intToIP(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}
