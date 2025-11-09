package main

import (
	"fmt"
	"os"
	"time"

	"github.com/FlameInTheDark/khm/internal/knownhosts"
	"github.com/FlameInTheDark/khm/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func runUI(knownHostsPath, version string) error {
	collection, err := knownhosts.ParseKnownHosts(knownHostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			collection = knownhosts.NewHostCollection(knownHostsPath)
		} else {
			return fmt.Errorf("failed to parse known_hosts: %w", err)
		}
	}

	model := ui.NewModel(collection, version)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}

func getKnownHostsPath() string {
	if path := os.Getenv("SSH_KNOWN_HOSTS"); path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "known_hosts"
	}
	return home + "/.ssh/known_hosts"
}

func listKnownHosts(knownHostsPath string) error {
	collection, err := knownhosts.ParseKnownHosts(knownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to parse known_hosts: %w", err)
	}

	fmt.Println("SSH Known Hosts:")
	fmt.Println("================")

	for _, addr := range collection.GetAllAddresses() {
		hosts := collection.GetHostsByAddress(addr)
		for i, host := range hosts {
			displayAddr := addr
			if host.IsHashed && host.HashValue != "" {
				displayAddr = host.HashValue
			} else if len(host.Addresses) > 0 {
				displayAddr = host.Addresses[0]
			}

			fmt.Printf("%d. %s (%s)\n", i+1, displayAddr, host.Type)
			if host.Comment != "" {
				fmt.Printf("   Comment: %s\n", host.Comment)
			}
		}
		fmt.Println()
	}

	return nil
}

func backupKnownHosts(sourcePath string) error {
	backupPath := sourcePath + ".backup." + fmt.Sprintf("%d", getTimestamp())

	if err := knownhosts.CopyFile(sourcePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("Backup created: %s\n", backupPath)
	return nil
}

func getTimestamp() int64 {
	return time.Now().Unix()
}

func stashHost(knownHostsPath, stashPath, host string) error {
	if knownHostsPath == "" {
		knownHostsPath = getKnownHostsPath()
	}

	collection, err := knownhosts.ParseKnownHosts(knownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to parse known_hosts: %w", err)
	}

	if err := collection.StashAddressWithPath(host, stashPath); err != nil {
		return fmt.Errorf("failed to stash host %q: %w", host, err)
	}

	return nil
}

func deleteHost(knownHostsPath, host string) error {
	if knownHostsPath == "" {
		knownHostsPath = getKnownHostsPath()
	}

	collection, err := knownhosts.ParseKnownHosts(knownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to parse known_hosts: %w", err)
	}

	if err := collection.RemoveAllHosts(host); err != nil {
		return fmt.Errorf("failed to delete host %q: %w", host, err)
	}

	if err := collection.SaveToFile(collection.File); err != nil {
		return fmt.Errorf("failed to save known_hosts after delete: %w", err)
	}

	return nil
}
