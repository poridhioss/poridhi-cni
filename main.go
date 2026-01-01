package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/poridhioss/poridhi-cni/pkg/cni"
)

func main() {
	// Step 1: Load environment variables from the runtime
	envArgs, err := cni.LoadEnvArgs()
	if err != nil {
		exitWithError(1, "failed to load environment", err.Error())
	}

	// Step 2: Load network configuration from stdin
	conf, err := cni.LoadNetConf(os.Stdin)
	if err != nil {
		exitWithError(2, "failed to load config", err.Error())
	}

	// Step 3: Route to the appropriate command handler
	switch envArgs.Command {
	case "ADD":
		err = cmdAdd(envArgs, conf)
	case "DEL":
		err = cmdDel(envArgs, conf)
	case "CHECK":
		err = cmdCheck(envArgs, conf)
	case "VERSION":
		err = cmdVersion(conf)
	default:
		exitWithError(3, "unknown CNI command", envArgs.Command)
	}

	if err != nil {
		exitWithError(4, "command failed", err.Error())
	}
}

// cmdAdd handles the ADD command - sets up networking for a container
func cmdAdd(args *cni.EnvArgs, conf *cni.NetConf) error {
	// TODO: Implement actual networking in later labs
	// For now, return a minimal valid result
	result := &cni.Result{
		CNIVersion: conf.CNIVersion,
		// In later labs, we'll add:
		// - Interfaces (created veth pair)
		// - IPs (allocated from IPAM)
		// - Routes (default gateway)
		// - DNS configuration
	}
	return printResult(result)
}

// cmdDel handles the DEL command - cleans up networking for a container
func cmdDel(args *cni.EnvArgs, conf *cni.NetConf) error {
	// TODO: Implement cleanup in later labs
	// DEL should be idempotent - succeed even if nothing to delete
	return nil
}

// cmdCheck handles the CHECK command - verifies networking is correct
func cmdCheck(args *cni.EnvArgs, conf *cni.NetConf) error {
	// TODO: Implement health checking in later labs
	return nil
}

// cmdVersion handles the VERSION command - reports supported CNI versions
func cmdVersion(conf *cni.NetConf) error {
	version := map[string]interface{}{
		"cniVersion":        conf.CNIVersion,
		"supportedVersions": []string{"0.3.0", "0.4.0", "1.0.0"},
	}
	return printJSON(version)
}

// printResult outputs the CNI result to stdout
func printResult(result *cni.Result) error {
	return printJSON(result)
}

// printJSON marshals any value to JSON and writes to stdout
func printJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = os.Stdout.Write(data)
	return err
}

// exitWithError writes a CNI error to stderr and exits
func exitWithError(code int, msg, details string) {
	cniErr := cni.Error{
		Code:    code,
		Msg:     msg,
		Details: details,
	}
	data, _ := json.Marshal(cniErr)
	os.Stderr.Write(data)
	os.Exit(1)
}