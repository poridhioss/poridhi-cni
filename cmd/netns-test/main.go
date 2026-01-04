package main

import (
	"fmt"
	"net"
	"os"

	"github.com/poridhioss/poridhi-cni/pkg/netns"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		if len(os.Args) < 3 {
			fmt.Println("Usage: netns-test create <name>")
			os.Exit(1)
		}
		createNS(os.Args[2])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: netns-test delete <name>")
			os.Exit(1)
		}
		deleteNS(os.Args[2])

	case "exec":
		if len(os.Args) < 3 {
			fmt.Println("Usage: netns-test exec <name>")
			os.Exit(1)
		}
		execInNS(os.Args[2])

	case "list":
		listInterfaces()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: netns-test <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  create <name>  - Create a new network namespace")
	fmt.Println("  delete <name>  - Delete a network namespace")
	fmt.Println("  exec <name>    - Execute and list interfaces in namespace")
	fmt.Println("  list           - List interfaces in current namespace")
}

func createNS(name string) {
	ns, err := netns.CreateNamed(name)
	if err != nil {
		fmt.Printf("Failed to create namespace: %v\n", err)
		os.Exit(1)
	}
	defer ns.Close()

	fmt.Printf("Created namespace: %s\n", ns.Path())
	fmt.Printf("File descriptor: %d\n", ns.Fd())
}

func deleteNS(name string) {
	if err := netns.Delete(name); err != nil {
		fmt.Printf("Failed to delete namespace: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted namespace: %s\n", name)
}

func execInNS(name string) {
	nsPath := fmt.Sprintf("/var/run/netns/%s", name)

	fmt.Println("Current namespace interfaces:")
	listInterfaces()

	fmt.Printf("\nSwitching to namespace: %s\n", name)
	fmt.Println("---")

	err := netns.WithNetNS(nsPath, func() error {
		fmt.Println("Inside namespace, interfaces:")
		listInterfaces()
		return nil
	})

	if err != nil {
		fmt.Printf("Failed to exec in namespace: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("---")
	fmt.Println("Back in original namespace, interfaces:")
	listInterfaces()
}

func listInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}

	if len(ifaces) == 0 {
		fmt.Println("  (no interfaces)")
		return
	}

	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		fmt.Printf("  %s: %v (flags: %v)\n", iface.Name, addrs, iface.Flags)
	}
}