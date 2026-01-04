package netns

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

const (
	netnsPath = "/var/run/netns"
)

// NetNS represents a network namespace
type NetNS struct {
	path string
	fd   int
}

// GetFromPath opens a namespace from a file path
func GetFromPath(path string) (*NetNS, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open ns %s: %w", path, err)
	}

	return &NetNS{
		path: path,
		fd:   fd,
	}, nil
}

// GetCurrent returns the current thread's network namespace
func GetCurrent() (*NetNS, error) {
	return GetFromPath("/proc/self/ns/net")
}

// GetFromPid returns the network namespace of a process
func GetFromPid(pid int) (*NetNS, error) {
	return GetFromPath(fmt.Sprintf("/proc/%d/ns/net", pid))
}

// Path returns the filesystem path of the namespace
func (ns *NetNS) Path() string {
	return ns.path
}

// Fd returns the file descriptor
func (ns *NetNS) Fd() int {
	return ns.fd
}

// Close closes the namespace file descriptor
func (ns *NetNS) Close() error {
	if ns.fd >= 0 {
		err := unix.Close(ns.fd)
		ns.fd = -1
		return err
	}
	return nil
}

// Set switches the current thread to this namespace
func (ns *NetNS) Set() error {
	return unix.Setns(ns.fd, unix.CLONE_NEWNET)
}

// CreateNamed creates a named network namespace
func CreateNamed(name string) (*NetNS, error) {
	nsPath := filepath.Join(netnsPath, name)

	// Create the netns directory if it doesn't exist
	if err := os.MkdirAll(netnsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create netns dir: %w", err)
	}

	// Create empty file to mount to
	f, err := os.Create(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create ns file: %w", err)
	}
	f.Close()

	// Lock thread for namespace operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current namespace to return to later
	origNS, err := GetCurrent()
	if err != nil {
		os.Remove(nsPath)
		return nil, fmt.Errorf("failed to get current ns: %w", err)
	}
	defer origNS.Close()

	// Create new namespace by unsharing
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		os.Remove(nsPath)
		return nil, fmt.Errorf("failed to unshare: %w", err)
	}

	// Bind mount the new namespace to the file
	if err := unix.Mount("/proc/self/ns/net", nsPath, "none", unix.MS_BIND, ""); err != nil {
		origNS.Set()
		os.Remove(nsPath)
		return nil, fmt.Errorf("failed to mount ns: %w", err)
	}

	// Return to original namespace
	if err := origNS.Set(); err != nil {
		return nil, fmt.Errorf("failed to return to original ns: %w", err)
	}

	return GetFromPath(nsPath)
}

// Delete removes a named namespace
func Delete(name string) error {
	nsPath := filepath.Join(netnsPath, name)

	// Unmount the namespace
	if err := unix.Unmount(nsPath, unix.MNT_DETACH); err != nil {
		// Ignore if not mounted or doesn't exist
		if err != unix.EINVAL && err != unix.ENOENT {
			return fmt.Errorf("failed to unmount ns: %w", err)
		}
	}

	// Remove the file
	if err := os.Remove(nsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove ns file: %w", err)
	}

	return nil
}

// WithNetNS executes a function within a network namespace
// This is the safe way to run code in a different namespace
func WithNetNS(nsPath string, fn func() error) error {
	// Lock the OS thread so namespace changes only affect this goroutine
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Get current namespace to return to later
	origNS, err := GetCurrent()
	if err != nil {
		return fmt.Errorf("failed to get current ns: %w", err)
	}
	defer origNS.Close()

	// Open target namespace
	targetNS, err := GetFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to open target ns: %w", err)
	}
	defer targetNS.Close()

	// Switch to target namespace
	if err := targetNS.Set(); err != nil {
		return fmt.Errorf("failed to enter ns: %w", err)
	}

	// Execute the function in the target namespace
	fnErr := fn()

	// ALWAYS try to return to original namespace
	if err := origNS.Set(); err != nil {
		// This is bad - we're stuck in wrong namespace
		return fmt.Errorf("CRITICAL: failed to return to original ns: %w (fn error: %v)", err, fnErr)
	}

	return fnErr
}

// WithNetNSByPath is like WithNetNS but opens namespace by path
func WithNetNSByPath(path string, fn func() error) error {
	return WithNetNS(path, fn)
}