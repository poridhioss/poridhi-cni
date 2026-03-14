package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/poridhioss/poridhi-cni/pkg/docker"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker integration commands",
}

var dockerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Docker containers with network info",
	RunE:  runDockerList,
}

var dockerAttachCmd = &cobra.Command{
	Use:   "attach <container-id>",
	Short: "Attach CNI networking to a Docker container",
	Args:  cobra.ExactArgs(1),
	RunE:  runDockerAttach,
}

var dockerDetachCmd = &cobra.Command{
	Use:   "detach <container-id>",
	Short: "Detach CNI networking from a Docker container",
	Args:  cobra.ExactArgs(1),
	RunE:  runDockerDetach,
}

var dockerInspectCmd = &cobra.Command{
	Use:   "inspect <container-id>",
	Short: "Inspect container network configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runDockerInspect,
}

func init() {
	rootCmd.AddCommand(dockerCmd)
	dockerCmd.AddCommand(dockerListCmd)
	dockerCmd.AddCommand(dockerAttachCmd)
	dockerCmd.AddCommand(dockerDetachCmd)
	dockerCmd.AddCommand(dockerInspectCmd)
}

func runDockerList(cmd *cobra.Command, args []string) error {
	if !docker.IsDockerAvailable() {
		return fmt.Errorf("docker is not available")
	}

	containers, err := docker.ListContainers(true)
	if err != nil {
		return err
	}

	fmt.Println("\nDocker Containers")
	fmt.Println(strings.Repeat("=", 80))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CONTAINER ID\tNAME\tSTATE\tPID\tIP ADDRESS")

	for _, c := range containers {
		ip := c.IPAddress
		if ip == "" {
			ip = "-"
		}
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		pidStr := "-"
		if c.PID > 0 {
			pidStr = fmt.Sprintf("%d", c.PID)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			shortID, c.Name, c.State, pidStr, ip)
	}
	w.Flush()

	return nil
}

func runDockerAttach(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	info, err := docker.GetContainerInfo(containerID)
	if err != nil {
		return err
	}

	if info.State != "running" {
		return fmt.Errorf("container is not running (state: %s)", info.State)
	}

	fmt.Printf("Attaching CNI networking to container %s...\n", info.Name)
	fmt.Printf("Network namespace: %s\n", info.NetNSPath)

	exePath, _ := os.Executable()
	scriptPath := filepath.Join(filepath.Dir(exePath), "docker-cni.sh")

	script := exec.Command("sudo", scriptPath, "add", containerID)
	script.Stdout = os.Stdout
	script.Stderr = os.Stderr

	return script.Run()
}

func runDockerDetach(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	info, err := docker.GetContainerInfo(containerID)
	if err != nil {
		return err
	}

	fmt.Printf("Detaching CNI networking from container %s...\n", info.Name)

	exePath, _ := os.Executable()
	scriptPath := filepath.Join(filepath.Dir(exePath), "docker-cni.sh")

	script := exec.Command("sudo", scriptPath, "del", containerID)
	script.Stdout = os.Stdout
	script.Stderr = os.Stderr

	return script.Run()
}

func runDockerInspect(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	info, err := docker.GetContainerInfo(containerID)
	if err != nil {
		return err
	}

	shortID := info.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	fmt.Printf("\nContainer: %s (%s)\n", info.Name, shortID)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("State:     %s\n", info.State)
	fmt.Printf("PID:       %d\n", info.PID)
	fmt.Printf("NetNS:     %s\n", info.NetNSPath)
	fmt.Printf("IP:        %s\n", info.IPAddress)
	fmt.Println()

	if info.PID > 0 {
		fmt.Println("Network Interfaces (inside container):")
		nsenter := exec.Command("nsenter", fmt.Sprintf("--net=%s", info.NetNSPath), "ip", "addr")
		nsenter.Stdout = os.Stdout
		nsenter.Run()
	}

	return nil
}
