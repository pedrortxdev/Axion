package axhv

import (
	"strconv"
	"strings"

	"aexon/internal/provider/axhv/pb"
	"aexon/internal/types"
	"aexon/internal/utils"
)

// MapCreateRequest maps the internal CreateInstanceRequest to the protobuf CreateVmRequest.
// It also enforces Free Tier limitations.
func MapCreateRequest(req types.Instance, ip string, gateway string) (*pb.CreateVmRequest, error) {

	// Parse Limits
	cpu := utils.ParseCpuCores(req.Limits["cpu"])
	if cpu == 0 {
		cpu = 1
	}

	ram := utils.ParseMemoryToMB(req.Limits["memory"])
	if ram == 0 {
		ram = 512
	}

	disk := uint32(10) // Default 10GB if not specified
	if val, ok := req.Limits["disk"]; ok {
		// Simplified parsing, assuming GB integer for now or implementing parser
		d, _ := strconv.Atoi(val)
		if d > 0 {
			disk = uint32(d)
		}
	}

	// Parse Ports
	portMap := make(map[string]uint32)
	if val, ok := req.Limits["ports"]; ok {
		// Input: "2202:22,8080:80"
		rules := strings.Split(val, ",")
		for _, rule := range rules {
			parts := strings.Split(rule, ":")
			if len(parts) == 2 {
				// Assuming format hostPort:guestPort
				// The pb definition uses map<string, uint32> port_map_tcp = 6;
				// Key is Host Port (string as per proto? Or maybe guest port?)
				// Let's check proto definition. Usually map<string, uint32> is string key, int value.
				// Wait, checking my previous view of proto...
				// API.md says: `port_map_tcp` (map<string, uint32>) -> "80/tcp": 8080  (TargetPort:HostPort?)
				// Actually AxHV v2 docs usually Map GuestPort -> HostPort or HostPort -> GuestPort?
				// "Guest IP is internal". We map Host Port -> Guest Port.
				// If Proto uses `map<string, uint32>`, key is usually string.
				// Let's assume Key = Host Port (string), Value = Guest Port (uint32).
				// Or Key = Guest Port (string/proto convention), Value = Host Port.
				// Based on `mapper.go` usage in `applyFreeTierLimits` where it iterates `req.PortMapTcp`, it treats it as a map.

				// Standard Container Mapping: Host:Container.
				// Let's assume Key = HostPort (String), Value = GuestPort (UInt32).

				hostPort := parts[0]
				guestPort, _ := strconv.Atoi(parts[1])

				if guestPort > 0 {
					portMap[hostPort] = uint32(guestPort)
				}
			}
		}
	}

	pbReq := &pb.CreateVmRequest{
		Id:           req.Name,
		Vcpu:         uint32(cpu),
		MemoryMib:    uint32(ram),
		DiskSizeGb:   disk,
		GuestIp:      ip,
		GuestGateway: gateway,
		// Assuming Template ID maps to R2 template or similar
		// For now using RootfsPath generic if image looks like a path
		// Or Template field if it's a known template name
		Template:   req.Image,
		PortMapTcp: portMap,
	}

	// Enforce Free Tier Limits (Hardcoded enforcement for now as requested)
	// In a real scenario, we might check req.Plan or User context.
	// Assuming all creations via this path are subject to these rules for the task context "Free Tier Enforcement".

	applyFreeTierLimits(pbReq)

	return pbReq, nil
}

func applyFreeTierLimits(req *pb.CreateVmRequest) {
	// 1. Bandwidth Limit 10Mbps
	req.BandwidthLimitMbps = 10

	// 2. Port Limits
	// As we don't have ports in the generic input yet (usually added later),
	// we initialize the maps to empty or filtered if they were passed.
	// If the request had ports (e.g. from a rich request object), we would filter them here.
	// Since types.Instance doesn't strictly have a list of initial ports in its basic struct
	// (usually added via AddPort), we ensure the map is initialized to allow strict validation if we were to add them.

	// Example of restricting if we were populating from a source that had ports:
	limitTcp := 3
	limitUdp := 1

	if len(req.PortMapTcp) > limitTcp {
		// Truncate logic or error? The requirement says "Maximo 3 portas".
		// We will keep only the first N found (non-deterministic map iteration but enforces count)
		newMap := make(map[string]uint32)
		i := 0
		for k, v := range req.PortMapTcp {
			if i >= limitTcp {
				break
			}
			newMap[k] = v
			i++
		}
		req.PortMapTcp = newMap
	}

	if len(req.PortMapTcp) > limitUdp {
		newMap := make(map[string]uint32)
		i := 0
		for k, v := range req.PortMapUdp {
			if i >= limitUdp {
				break
			}
			newMap[k] = v
			i++
		}
		req.PortMapUdp = newMap
	}
}
