package iptables

import (
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

const (
	natTable    = "nat"
	filterTable = "filter"

	postroutingChain = "POSTROUTING"
	forwardChain     = "FORWARD"
)

// Manager manages iptables rules for CNI
type Manager struct {
	ipt *iptables.IPTables
}

// NewManager creates a new iptables manager
func NewManager() (*Manager, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iptables: %w", err)
	}

	return &Manager{ipt: ipt}, nil
}

// SetupNAT configures MASQUERADE for container network traffic
func (m *Manager) SetupNAT(subnet, bridgeName string) error {
	// -A POSTROUTING -s 10.244.0.0/24 ! -o cni0 -j MASQUERADE
	rule := []string{
		"-s", subnet,
		"!", "-o", bridgeName,
		"-j", "MASQUERADE",
	}

	exists, err := m.ipt.Exists(natTable, postroutingChain, rule...)
	if err != nil {
		return fmt.Errorf("failed to check NAT rule: %w", err)
	}

	if !exists {
		if err := m.ipt.Append(natTable, postroutingChain, rule...); err != nil {
			return fmt.Errorf("failed to add MASQUERADE rule: %w", err)
		}
	}

	return nil
}


// SetupForward configures FORWARD chain rules for bridge traffic
func (m *Manager) SetupForward(subnet, bridgeName string) error {
	// Allow forwarding from bridge
	// -A FORWARD -i cni0 -j ACCEPT
	rule1 := []string{"-i", bridgeName, "-j", "ACCEPT"}
	if err := m.ensureRule(filterTable, forwardChain, rule1); err != nil {
		return fmt.Errorf("failed to add FORWARD input rule: %w", err)
	}

	// Allow forwarding to bridge
	// -A FORWARD -o cni0 -j ACCEPT
	rule2 := []string{"-o", bridgeName, "-j", "ACCEPT"}
	if err := m.ensureRule(filterTable, forwardChain, rule2); err != nil {
		return fmt.Errorf("failed to add FORWARD output rule: %w", err)
	}

	return nil
}

// ensureRule adds a rule if it doesn't exist (idempotent)
func (m *Manager) ensureRule(table, chain string, rule []string) error {
	exists, err := m.ipt.Exists(table, chain, rule...)
	if err != nil {
		return fmt.Errorf("failed to check rule: %w", err)
	}

	if !exists {
		if err := m.ipt.Append(table, chain, rule...); err != nil {
			return fmt.Errorf("failed to add rule: %w", err)
		}
	}

	return nil
}


// CleanupNAT removes NAT rules for container network
func (m *Manager) CleanupNAT(subnet, bridgeName string) error {
	rule := []string{
		"-s", subnet,
		"!", "-o", bridgeName,
		"-j", "MASQUERADE",
	}

	exists, err := m.ipt.Exists(natTable, postroutingChain, rule...)
	if err != nil {
		return nil // Ignore errors during cleanup
	}

	if exists {
		m.ipt.Delete(natTable, postroutingChain, rule...)
	}

	return nil
}

// CleanupForward removes FORWARD rules for bridge traffic
func (m *Manager) CleanupForward(bridgeName string) error {
	rules := [][]string{
		{"-i", bridgeName, "-j", "ACCEPT"},
		{"-o", bridgeName, "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		exists, err := m.ipt.Exists(filterTable, forwardChain, rule...)
		if err != nil {
			continue
		}
		if exists {
			m.ipt.Delete(filterTable, forwardChain, rule...)
		}
	}

	return nil
}

// ListNATRules returns all NAT POSTROUTING rules
func (m *Manager) ListNATRules() ([]string, error) {
	rules, err := m.ipt.List(natTable, postroutingChain)
	if err != nil {
		return nil, fmt.Errorf("failed to list NAT rules: %w", err)
	}
	return rules, nil
}

// ListForwardRules returns all FORWARD rules
func (m *Manager) ListForwardRules() ([]string, error) {
	rules, err := m.ipt.List(filterTable, forwardChain)
	if err != nil {
		return nil, fmt.Errorf("failed to list FORWARD rules: %w", err)
	}
	return rules, nil
}

// RuleInfo represents an iptables rule for display
type RuleInfo struct {
	Table string
	Chain string
	Rule  string
}

// GetCNIRules returns all CNI-related iptables rules
func (m *Manager) GetCNIRules() ([]RuleInfo, error) {
	var result []RuleInfo

	// Check NAT table
	natRules, err := m.ipt.List(natTable, postroutingChain)
	if err == nil {
		for _, rule := range natRules {
			if strings.Contains(rule, "MASQUERADE") || strings.Contains(rule, "10.244") {
				result = append(result, RuleInfo{
					Table: natTable,
					Chain: postroutingChain,
					Rule:  rule,
				})
			}
		}
	}

	// Check FORWARD chain
	fwdRules, err := m.ipt.List(filterTable, forwardChain)
	if err == nil {
		for _, rule := range fwdRules {
			if strings.Contains(rule, "cni0") {
				result = append(result, RuleInfo{
					Table: filterTable,
					Chain: forwardChain,
					Rule:  rule,
				})
			}
		}
	}

	return result, nil
}
