package cni

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// EnvArgs holds CNI environment variables passed by the runtime
type EnvArgs struct {
	Command     string
	ContainerID string
	NetNS       string
	IfName      string
	Path        string
	Args        string
}

// LoadEnvArgs reads CNI environment variables
func LoadEnvArgs() (*EnvArgs, error) {
	args := &EnvArgs{
		Command:     os.Getenv("CNI_COMMAND"),
		ContainerID: os.Getenv("CNI_CONTAINERID"),
		NetNS:       os.Getenv("CNI_NETNS"),
		IfName:      os.Getenv("CNI_IFNAME"),
		Path:        os.Getenv("CNI_PATH"),
		Args:        os.Getenv("CNI_ARGS"),
	}

	// CNI_COMMAND is always required
	if args.Command == "" {
		return nil, fmt.Errorf("CNI_COMMAND environment variable is not set")
	}

	return args, nil
}

// LoadNetConf reads and parses CNI configuration from stdin
func LoadNetConf(r io.Reader) (*NetConf, error) {
	// Read all data from stdin
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from stdin: %w", err)
	}

	// Parse JSON into NetConf struct
	var conf NetConf
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse CNI config: %w", err)
	}

	// Validate required fields
	if conf.CNIVersion == "" {
		return nil, fmt.Errorf("cniVersion is required in config")
	}
	if conf.Name == "" {
		return nil, fmt.Errorf("name is required in config")
	}

	return &conf, nil
}