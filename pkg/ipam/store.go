package ipam

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// Store manages IP allocations on disk
type Store struct {
	dataDir string
	network string
}

// NewStore creates a new IPAM store
func NewStore(dataDir, network string) *Store {
	return &Store{
		dataDir: dataDir,
		network: network,
	}
}

func (s *Store) networkDir() string {
	return filepath.Join(s.dataDir, s.network)
}

func (s *Store) lockPath() string {
	return filepath.Join(s.networkDir(), "lock")
}

func (s *Store) lastIPPath() string {
	return filepath.Join(s.networkDir(), "last_reserved_ip.0")
}

func (s *Store) ipPath(ip net.IP) string {
	return filepath.Join(s.networkDir(), ip.String())
}

// Lock acquires exclusive lock on the store
func (s *Store) Lock() (*os.File, error) {
	if err := os.MkdirAll(s.networkDir(), 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(s.lockPath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return f, nil
}

// Unlock releases the lock
func (s *Store) Unlock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return err
	}
	return f.Close()
}

// GetLastIP reads the last allocated IP
func (s *Store) GetLastIP() (net.IP, error) {
	data, err := os.ReadFile(s.lastIPPath())
	if os.IsNotExist(err) {
		return nil, nil // No previous allocation
	}
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(string(data))
	if ip == nil {
		return nil, fmt.Errorf("invalid IP in last_reserved_ip: %s", string(data))
	}

	return ip, nil
}

// SetLastIP writes the last allocated IP
func (s *Store) SetLastIP(ip net.IP) error {
	return os.WriteFile(s.lastIPPath(), []byte(ip.String()), 0644)
}

// Reserve marks an IP as allocated
func (s *Store) Reserve(ip net.IP, containerID string) error {
	return os.WriteFile(s.ipPath(ip), []byte(containerID), 0644)
}

// Release frees an allocated IP
func (s *Store) Release(ip net.IP) error {
	err := os.Remove(s.ipPath(ip))
	if os.IsNotExist(err) {
		return nil // Already released, that's fine
	}
	return err
}

// IsAllocated checks if an IP is allocated
func (s *Store) IsAllocated(ip net.IP) bool {
	_, err := os.Stat(s.ipPath(ip))
	return err == nil
}

// GetContainerByIP returns the container ID for an IP
func (s *Store) GetContainerByIP(ip net.IP) (string, error) {
	data, err := os.ReadFile(s.ipPath(ip))
	if err != nil {
		return "", err
	}
	return string(data), nil
}