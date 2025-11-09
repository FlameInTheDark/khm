package main

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	rootCmd *cobra.Command
)

func init() {

	rootCmd = &cobra.Command{

		Use: "khm",

		Short: "Manage SSH known_hosts files with a TUI",
		Long:  `CLI tool for managing SSH known_hosts files via a terminal user interface.`,

		Version: version,

		Run: func(cmd *cobra.Command, args []string) {

			path, _ := cmd.Flags().GetString("file")

			if path == "" {

				path = getKnownHostsPath()

			}

			if err := runUI(path); err != nil {

				log.Fatal(err)

			}

		},
	}

	rootCmd.PersistentFlags().StringP("file", "f", "", "Path to known_hosts file (overrides SSH_KNOWN_HOSTS and default)")

	rootCmd.AddCommand(

		uiCmd(),

		listCmd(),

		backupCmd(),

		stashCmd(),

		deleteCmd(),
	)

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all known hosts",
		Run: func(cmd *cobra.Command, args []string) {
			path, _ := cmd.Flags().GetString("file")
			if path == "" {
				path = getKnownHostsPath()
			}
			if err := listKnownHosts(path); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a backup of known_hosts file",
		Run: func(cmd *cobra.Command, args []string) {
			path, _ := cmd.Flags().GetString("file")
			if path == "" {
				path = getKnownHostsPath()
			}
			if err := backupKnownHosts(path); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func uiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Launch the TUI interface",
		Run: func(cmd *cobra.Command, args []string) {
			path, _ := cmd.Flags().GetString("file")
			if path == "" {
				path = getKnownHostsPath()
			}
			if err := runUI(path); err != nil {
				log.Fatal(err)
			}
		},
	}
}

// stashCmd moves (stashes) all keys for the given host/address into stash_hosts
// next to the known_hosts file, avoiding duplicates in stash.
func stashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stash <host>",
		Short: "Stash all keys for a host into a stash_hosts file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			host := args[0]
			if host == "" {
				log.Fatal("host is required")
			}

			knownPath, _ := cmd.Flags().GetString("file")
			if knownPath == "" {
				knownPath = getKnownHostsPath()
			}

			stashPath, _ := cmd.Flags().GetString("stash-file")

			if err := stashHost(knownPath, stashPath, host); err != nil {
				log.Fatal(err)
			}
		},
	}

	// Optional custom stash file path; if not set, defaults to stash_hosts next to known_hosts.
	cmd.Flags().StringP("stash-file", "s", "", "Path to stash file (default: stash_hosts next to known_hosts)")

	return cmd
}

// deleteCmd removes all keys for the given host/address from known_hosts.
func deleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <host>",
		Short: "Delete all keys for a host from known_hosts",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			host := args[0]
			if host == "" {
				log.Fatal("host is required")
			}

			path, _ := cmd.Flags().GetString("file")
			if path == "" {
				path = getKnownHostsPath()
			}

			if err := deleteHost(path, host); err != nil {
				log.Fatal(err)
			}
		},
	}
}
