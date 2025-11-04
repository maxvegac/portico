package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/cmd/portico/commands"
)

var rootCmd = &cobra.Command{
	Use:   "portico",
	Short: "Portico - PaaS platform for managing applications",
	Long:  `Portico is a PaaS platform, using Caddy as reverse proxy and Docker Compose for applications.`,
}

func main() {
	// Check for auto-updates before running any command
	commands.CheckAutoUpdate()

	// Create commands
	versionCmd := commands.NewVersionCmd()
	updateCmd := commands.NewUpdateCmd()
	checkUpdateCmd := commands.NewCheckUpdateCmd()
	autoUpdateCmd := commands.NewAutoUpdateCmd()

	// App commands - add directly to root (without "apps" prefix)
	createCmd := commands.NewAppsCreateCmd()
	createCmd.Use = "create [app-name]"
	listCmd := commands.NewAppsListCmd()
	listCmd.Use = "list"
	resetCmd := commands.NewAppsResetCmd()
	resetCmd.Use = "reset [app-name]"
	destroyCmd := commands.NewAppsDestroyCmd()
	destroyCmd.Use = "destroy [app-name]"
	upCmd := commands.NewAppsUpCmd()
	upCmd.Use = "up [app-name]"
	downCmd := commands.NewAppsDownCmd()
	downCmd.Use = "down [app-name]"
	cdCmd := commands.NewAppsCdCmd()
	cdCmd.Use = "cd [app-name]"
	preserveCmd := commands.NewAppsPreserveCmd()
	preserveCmd.Use = "preserve [app-name]"
	execCmd := commands.NewAppsExecCmd()
	execCmd.Use = "exec [app-name] [[service] [command...]]"
	shellCmd := commands.NewAppsShellCmd()
	shellCmd.Use = "shell [app-name] [[service] [shell]]"
	statusCmd := commands.NewAppsStatusCmd()
	statusCmd.Use = "status [app-name]"

	// Domains command
	domainsCmd := commands.NewDomainsCmd()
	domainsCmd.AddCommand(commands.NewDomainsAddCmd())
	domainsCmd.AddCommand(commands.NewDomainsRemoveCmd())

	// Ports commands (port mappings)
	portsCmd := commands.NewPortsCmd()
	portsCmd.AddCommand(commands.NewPortsAddCmd())
	portsCmd.AddCommand(commands.NewPortsDeleteCmd())
	portsCmd.AddCommand(commands.NewPortsListCmd())

	// Storage commands (volume mounts)
	storageCmd := commands.NewStorageCmd()
	storageCmd.AddCommand(commands.NewStorageAddCmd())
	storageCmd.AddCommand(commands.NewStorageDeleteCmd())
	storageCmd.AddCommand(commands.NewStorageListCmd())

	// Add flags to update command
	updateCmd.Flags().Bool("dev", false, "Check for development releases instead of stable releases")
	checkUpdateCmd.Flags().Bool("dev", false, "Check for development releases instead of stable releases")
	autoUpdateCmd.Flags().Bool("enable", false, "Enable automatic updates")
	autoUpdateCmd.Flags().Bool("disable", false, "Disable automatic updates")
	autoUpdateCmd.Flags().Bool("status", false, "Show auto-update status")

	// Add commands to root
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(checkUpdateCmd)
	rootCmd.AddCommand(autoUpdateCmd)

	// App commands
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(cdCmd)
	rootCmd.AddCommand(preserveCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(domainsCmd)
	rootCmd.AddCommand(portsCmd)
	rootCmd.AddCommand(storageCmd)

	// Addons commands
	addonsCmd := commands.NewAddonsCmd()
	addonsCmd.AddCommand(commands.NewAddonCreateCmd())
	rootCmd.AddCommand(addonsCmd)

	// Service commands
	rootCmd.AddCommand(commands.NewServiceCmd())

	// SSH commands (for managing git deployment keys)
	rootCmd.AddCommand(commands.NewSSHCmd())

	// Init command (for extracting embedded static files)
	rootCmd.AddCommand(commands.NewInitCmd())

	// Git commands (internal)
	rootCmd.AddCommand(commands.NewGitReceiveCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
