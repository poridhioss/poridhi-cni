package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ContainerInfo holds Docker container information
type ContainerInfo struct {
	ID        string
	Name      string
	PID       int
	NetNSPath string
	IPAddress string
	State     string
}

// GetContainerInfo retrieves information about a Docker container
func GetContainerInfo(containerID string) (*ContainerInfo, error) {
	cmd := exec.Command("docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspectResult []map[string]interface{}
	if err := json.Unmarshal(output, &inspectResult); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(inspectResult) == 0 {
		return nil, fmt.Errorf("container not found")
	}

	container := inspectResult[0]

	info := &ContainerInfo{
		ID: containerID,
	}

	// Get name
	if name, ok := container["Name"].(string); ok {
		info.Name = strings.TrimPrefix(name, "/")
	}

	// Get state and PID
	if state, ok := container["State"].(map[string]interface{}); ok {
		if pid, ok := state["Pid"].(float64); ok {
			info.PID = int(pid)
			info.NetNSPath = fmt.Sprintf("/proc/%d/ns/net", info.PID)
		}
		if status, ok := state["Status"].(string); ok {
			info.State = status
		}
	}

	// Get IP address (if any)
	if networks, ok := container["NetworkSettings"].(map[string]interface{}); ok {
		if ipAddr, ok := networks["IPAddress"].(string); ok && ipAddr != "" {
			info.IPAddress = ipAddr
		}
	}

	return info, nil
}

// GetContainerPID returns the PID of a Docker container
func GetContainerPID(containerID string) (int, error) {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get container PID: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID: %w", err)
	}

	return pid, nil
}

// GetNetNSPath returns the network namespace path for a container
func GetNetNSPath(containerID string) (string, error) {
	pid, err := GetContainerPID(containerID)
	if err != nil {
		return "", err
	}

	if pid == 0 {
		return "", fmt.Errorf("container is not running")
	}

	return fmt.Sprintf("/proc/%d/ns/net", pid), nil
}

// ListContainers returns all Docker containers
func ListContainers(all bool) ([]ContainerInfo, error) {
	args := []string{"ps", "--format", "{{.ID}}"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	for _, id := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if id == "" {
			continue
		}
		info, err := GetContainerInfo(id)
		if err != nil {
			continue
		}
		containers = append(containers, *info)
	}

	return containers, nil
}

// IsDockerAvailable checks if Docker is available
func IsDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}
