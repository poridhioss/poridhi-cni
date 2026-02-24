package iptables

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// ConntrackEntry represents a connection tracking entry
type ConntrackEntry struct {
	Protocol  string
	State     string
	OrigSrc   string
	OrigDst   string
	OrigSPort string
	OrigDPort string
	ReplySrc  string
	ReplyDst  string
}

// GetConntrackEntries reads connection tracking entries using the conntrack command
// filterIP limits results to entries involving a specific IP (empty string for all)
func GetConntrackEntries(filterIP string) ([]ConntrackEntry, error) {
	out, err := exec.Command("conntrack", "-L").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run conntrack -L: %w (is conntrack installed?)", err)
	}

	var entries []ConntrackEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	for scanner.Scan() {
		line := scanner.Text()

		if filterIP != "" && !strings.Contains(line, filterIP) {
			continue
		}

		entry := parseConntrackLine(line)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}



func parseConntrackLine(line string) *ConntrackEntry {
	entry := &ConntrackEntry{}
	parts := strings.Fields(line)

	if len(parts) < 4 {
		return nil
	}

	// Protocol is the first field
	entry.Protocol = parts[0]

	// Parse key=value pairs
	srcCount := 0
	dstCount := 0

	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "src="):
			if srcCount == 0 {
				entry.OrigSrc = strings.TrimPrefix(part, "src=")
			} else {
				entry.ReplySrc = strings.TrimPrefix(part, "src=")
			}
			srcCount++
		case strings.HasPrefix(part, "dst="):
			if dstCount == 0 {
				entry.OrigDst = strings.TrimPrefix(part, "dst=")
			} else {
				entry.ReplyDst = strings.TrimPrefix(part, "dst=")
			}
			dstCount++
		case strings.HasPrefix(part, "sport=") && entry.OrigSPort == "":
			entry.OrigSPort = strings.TrimPrefix(part, "sport=")
		case strings.HasPrefix(part, "dport=") && entry.OrigDPort == "":
			entry.OrigDPort = strings.TrimPrefix(part, "dport=")
		}
	}

	// Check for state keywords
	for _, part := range parts {
		switch part {
		case "ESTABLISHED", "SYN_SENT", "SYN_RECV", "FIN_WAIT",
			"CLOSE_WAIT", "TIME_WAIT", "CLOSE", "UNREPLIED":
			entry.State = part
		}
	}

	return entry
}


