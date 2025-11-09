package knownhosts

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Host struct {
	Addresses []string
	Type      string

	Key string

	Comment string

	LineNumber int

	IsHashed bool

	HashValue string
}

type HostCollection struct {
	Hosts map[string][]*Host

	File string
}

func NewHostCollection(filePath string) *HostCollection {

	return &HostCollection{

		Hosts: make(map[string][]*Host),

		File: filePath,
	}

}

func ParseKnownHosts(filePath string) (*HostCollection, error) {
	if filePath == "" {
		filePath = getDefaultKnownHostsPath()
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open known_hosts file: %w", err)
	}
	defer file.Close()

	collection := NewHostCollection(filePath)
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		host := parseHostLine(line, lineNumber)
		if host != nil {
			collection.AddHost(host)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading known_hosts file: %w", err)
	}

	return collection, nil
}

func parseHostLine(line string, lineNumber int) *Host {

	parts := strings.Fields(line)

	if len(parts) < 3 {

		return nil

	}

	rawHosts := strings.Split(parts[0], ",")
	if len(rawHosts) == 0 {
		return nil
	}

	host := &Host{

		Addresses:  make([]string, 0, len(rawHosts)),
		LineNumber: lineNumber,
	}

	for _, h := range rawHosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		host.Addresses = append(host.Addresses, h)
	}

	if len(host.Addresses) == 0 {
		return nil
	}

	if strings.HasPrefix(host.Addresses[0], "|") {
		host.IsHashed = true
		host.HashValue = host.Addresses[0]
	}

	if len(parts) > 1 {

		host.Type = parts[1]

	}

	if len(parts) > 2 {

		host.Key = parts[2]

	}

	if len(parts) > 3 {

		host.Comment = strings.Join(parts[3:], " ")

	}

	return host

}

func (hc *HostCollection) AddHost(host *Host) {

	if host == nil || len(host.Addresses) == 0 {
		return
	}

	for _, addr := range host.Addresses {
		if _, exists := hc.Hosts[addr]; !exists {
			hc.Hosts[addr] = []*Host{}
		}
		hc.Hosts[addr] = append(hc.Hosts[addr], host)
	}
}

func (hc *HostCollection) GetHostsByAddress(address string) []*Host {

	return hc.Hosts[address]

}

func (hc *HostCollection) GetAllAddresses() []string {
	addresses := make([]string, 0, len(hc.Hosts))
	for addr := range hc.Hosts {
		addresses = append(addresses, addr)
	}
	return addresses
}

func (hc *HostCollection) RemoveHost(address string, index int) error {

	hosts, exists := hc.Hosts[address]

	if !exists || index < 0 || index >= len(hosts) {

		return fmt.Errorf("host not found")

	}

	hc.Hosts[address] = append(hosts[:index], hosts[index+1:]...)

	if len(hc.Hosts[address]) == 0 {

		delete(hc.Hosts, address)

	}

	return nil

}

func (hc *HostCollection) MoveHostToFile(address string, index int, targetFile string) error {

	hosts, exists := hc.Hosts[address]

	if !exists || index < 0 || index >= len(hosts) {

		return fmt.Errorf("host not found")

	}

	host := hosts[index]

	if _, err := os.Stat(targetFile); os.IsNotExist(err) {

		file, err := os.Create(targetFile)

		if err != nil {

			return fmt.Errorf("failed to create target file: %w", err)

		}

		file.Close()

	}

	file, err := os.OpenFile(targetFile, os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {

		return fmt.Errorf("failed to open target file: %w", err)

	}

	defer file.Close()

	line := formatKnownHostsLine(host)
	line += "\n"

	if _, err := file.WriteString(line); err != nil {

		return fmt.Errorf("failed to write to target file: %w", err)

	}

	return hc.RemoveHost(address, index)

}

func formatKnownHostsLine(host *Host) string {
	if host == nil {
		return ""
	}

	addressField := ""
	if len(host.Addresses) > 0 {
		addressField = strings.Join(host.Addresses, ",")
	}

	line := fmt.Sprintf("%s %s %s", addressField, host.Type, host.Key)
	if host.Comment != "" {
		line += " " + host.Comment
	}
	return line
}

func (hc *HostCollection) Save() error {
	return hc.SaveToFile(hc.File)
}

func (hc *HostCollection) RemoveAllHosts(address string) error {
	if _, exists := hc.Hosts[address]; !exists {
		return fmt.Errorf("host not found")
	}
	delete(hc.Hosts, address)
	return nil
}

func (hc *HostCollection) MoveAllHostsToFile(address string, targetFile string) error {

	hosts, exists := hc.Hosts[address]

	if !exists || len(hosts) == 0 {

		return fmt.Errorf("host not found")

	}

	if _, err := os.Stat(targetFile); os.IsNotExist(err) {

		file, err := os.Create(targetFile)

		if err != nil {

			return fmt.Errorf("failed to create target file: %w", err)

		}

		file.Close()

	}

	file, err := os.OpenFile(targetFile, os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {

		return fmt.Errorf("failed to open target file: %w", err)

	}

	defer file.Close()

	seen := make(map[*Host]bool)
	for _, host := range hosts {

		if seen[host] {
			continue
		}
		seen[host] = true

		line := formatKnownHostsLine(host)
		line += "\n"
		if _, err := file.WriteString(line); err != nil {

			return fmt.Errorf("failed to write to target file: %w", err)

		}

	}

	delete(hc.Hosts, address)

	return nil

}

func (hc *HostCollection) StashFilePath() string {
	if hc.File == "" {
		return ""
	}
	dir := filepath.Dir(hc.File)
	return filepath.Join(dir, "stash_hosts")
}

func (hc *HostCollection) StashAddress(address string) error {
	return hc.StashAddressWithPath(address, hc.StashFilePath())
}

func (hc *HostCollection) StashAddressWithPath(address, stashPath string) error {
	hosts, exists := hc.Hosts[address]
	if !exists || len(hosts) == 0 {
		return fmt.Errorf("host not found")
	}

	if stashPath == "" {
		return fmt.Errorf("stash path not available")
	}

	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		f, err := os.Create(stashPath)
		if err != nil {
			return fmt.Errorf("failed to create stash file: %w", err)
		}
		f.Close()
	}

	stash, err := ParseKnownHosts(stashPath)
	if err != nil {
		return fmt.Errorf("failed to parse stash_hosts: %w", err)
	}

	existing := make(map[string]struct{})
	for _, addrs := range stash.Hosts {
		for _, h := range addrs {
			key := stashKey(h)
			if key != "" {
				existing[key] = struct{}{}
			}
		}
	}

	f, err := os.OpenFile(stashPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open stash file: %w", err)
	}
	defer f.Close()

	seenHost := make(map[*Host]bool)
	for _, h := range hosts {
		if h == nil || seenHost[h] {
			continue
		}
		seenHost[h] = true

		key := stashKey(h)
		if key != "" {
			if _, dup := existing[key]; dup {
				// Already in stash, do not write again
				continue
			}
			existing[key] = struct{}{}
		}

		line := formatKnownHostsLine(h)
		if line == "" {
			continue
		}
		if _, err := f.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write to stash file: %w", err)
		}
	}

	delete(hc.Hosts, address)

	if err := hc.SaveToFile(hc.File); err != nil {
		return fmt.Errorf("failed to save known_hosts after stash: %w", err)
	}

	return nil
}

func (hc *HostCollection) UnstashAddress(address string) error {
	stashPath := hc.StashFilePath()
	if stashPath == "" {
		return fmt.Errorf("stash path not available")
	}

	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		return fmt.Errorf("stash file not found")
	}

	mainCol, err := ParseKnownHosts(hc.File)
	if err != nil {
		return fmt.Errorf("failed to parse known_hosts: %w", err)
	}

	stashCol, err := ParseKnownHosts(stashPath)
	if err != nil {
		return fmt.Errorf("failed to parse stash_hosts: %w", err)
	}

	existing := make(map[string]struct{})
	if currentHosts, ok := mainCol.Hosts[address]; ok {
		for _, h := range currentHosts {
			k := stashKey(h)
			if k != "" {
				existing[k] = struct{}{}
			}
		}
	}

	stashHosts, ok := stashCol.Hosts[address]
	if !ok || len(stashHosts) == 0 {
		return fmt.Errorf("no stashed entries for address")
	}

	for _, h := range stashHosts {
		if h == nil {
			continue
		}
		k := stashKey(h)
		if k != "" {
			if _, dup := existing[k]; dup {
				// Skip duplicate
				continue
			}
			existing[k] = struct{}{}
		}
		mainCol.AddHost(h)
	}

	delete(stashCol.Hosts, address)

	if err := mainCol.SaveToFile(mainCol.File); err != nil {
		return fmt.Errorf("failed to save known_hosts after unstash: %w", err)
	}
	if err := stashCol.SaveToFile(stashCol.File); err != nil {
		return fmt.Errorf("failed to save stash_hosts after unstash: %w", err)
	}

	return nil
}

func stashKey(h *Host) string {
	if h == nil {
		return ""
	}
	addrField := strings.Join(h.Addresses, ",")
	if addrField == "" && h.IsHashed && h.HashValue != "" {
		addrField = h.HashValue
	}
	if addrField == "" || h.Type == "" || h.Key == "" {
		return ""
	}
	return addrField + " " + h.Type + " " + h.Key
}

func (hc *HostCollection) SaveToFile(filePath string) error {

	// Create backup first, but only fail if the source file exists and backup truly fails.

	backupPath := filePath + ".backup"

	if err := CopyFile(filePath, backupPath); err != nil && !os.IsNotExist(err) {

		return fmt.Errorf("failed to create backup: %w", err)

	}

	file, err := os.Create(filePath)

	if err != nil {

		return fmt.Errorf("failed to create file: %w", err)

	}

	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write header

	header := "# SSH Known Hosts File\n"

	header += "# Managed by ssh-knownhosts-manager\n"

	if _, err := writer.WriteString(header + "\n"); err != nil {

		return fmt.Errorf("failed to write header: %w", err)

	}

	// To avoid duplicating multi-address hosts, track which *Host entries have been written.
	written := make(map[*Host]bool)

	// Collect and sort one representative address per host entry for deterministic output.

	// We iterate over all addresses, but only write each unique Host once using its original addresses.
	addresses := make([]string, 0, len(hc.Hosts))

	for addr := range hc.Hosts {

		addresses = append(addresses, addr)

	}

	sort.Strings(addresses)

	for _, addr := range addresses {

		for _, host := range hc.Hosts[addr] {

			if written[host] {
				continue
			}
			written[host] = true

			line := formatKnownHostsLine(host)
			if _, err := writer.WriteString(line + "\n"); err != nil {

				return fmt.Errorf("failed to write host: %w", err)

			}

		}

	}

	return writer.Flush()

}

func getDefaultKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "known_hosts"
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

func CopyFile(src, dst string) error {

	source, err := os.Open(src)

	if err != nil {

		if os.IsNotExist(err) {

			return err

		}

		return err

	}

	defer source.Close()

	destination, err := os.Create(dst)

	if err != nil {

		return err

	}

	defer destination.Close()

	buf := make([]byte, 1024)

	for {

		n, err := source.Read(buf)

		if n > 0 {

			if _, writeErr := destination.Write(buf[:n]); writeErr != nil {

				return writeErr

			}

		}

		if err != nil {

			break

		}

	}

	return nil

}
