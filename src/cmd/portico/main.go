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
	appsCmd := commands.NewAppsCmd()

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
	rootCmd.AddCommand(appsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
