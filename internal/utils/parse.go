package utils

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseMemoryToMB converte strings como "512MB", "1GB", "2G" para MegaBytes (int64).
// Retorna 0 se inválido ou vazio.
func ParseMemoryToMB(memStr string) int64 {
	if memStr == "" {
		return 0
	}

	// Remove espaços e converte para upper
	s := strings.ToUpper(strings.TrimSpace(memStr))

	re := regexp.MustCompile(`^(\d+)(MB|GB|G|M)?$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}

	unit := matches[2]

	switch unit {
	case "GB", "G":
		return val * 1024
	case "MB", "M", "":
		return val
	default:
		return 0
	}
}

// ParseCpuCores converte string de CPU para int.
func ParseCpuCores(cpuStr string) int {
	if cpuStr == "" {
		return 0
	}
	val, err := strconv.Atoi(cpuStr)
	if err != nil {
		return 0
	}
	return val
}

// ParseMemoryToBytes converte strings como "512MB", "1GB", "2G" para bytes (int64).
// Retorna 0 se inválido ou vazio.
func ParseMemoryToBytes(memStr string) int64 {
	if memStr == "" {
		return 0
	}

	// Remove espaços e converte para upper
	s := strings.ToUpper(strings.TrimSpace(memStr))

	re := regexp.MustCompile(`^(\d+)(B|K|KB|M|MB|G|GB)?$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(matches[2])

	switch unit {
	case "GB", "G":
		return val * 1024 * 1024 * 1024
	case "MB", "M":
		return val * 1024 * 1024
	case "KB", "K":
		return val * 1024
	case "B", "":
		return val
	default:
		return 0
	}
}
