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
	// Create commands
	versionCmd := commands.NewVersionCmd()
	appsCmd := commands.NewAppsCmd()
	appsListCmd := commands.NewAppsListCmd()
	appsCreateCmd := commands.NewAppsCreateCmd()
	appsDeployCmd := commands.NewAppsDeployCmd()
	appsDestroyCmd := commands.NewAppsDestroyCmd()

	// Add commands to root
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(appsCmd)

	// Add subcommands to apps
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsCreateCmd)
	appsCmd.AddCommand(appsDeployCmd)
	appsCmd.AddCommand(appsDestroyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
