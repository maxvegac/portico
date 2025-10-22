package main

import (
	"fmt"
	"os"

	"github.com/portico/portico/internal/app"
	"github.com/portico/portico/internal/config"
	"github.com/portico/portico/internal/proxy"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "portico",
	Short: "Portico - PaaS platform for managing applications",
	Long:  `Portico is a PaaS platform, using Caddy as reverse proxy and Docker Compose for applications.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Portico",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Portico v0.1.0")
	},
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage applications",
	Long:  `Manage applications deployed on Portico platform.`,
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all applications",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Listing applications...")
		// TODO: Implement app listing
	},
}

var appsCreateCmd = &cobra.Command{
	Use:   "create [app-name]",
	Short: "Create a new application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Creating application: %s\n", appName)
		
		// Load config
		config, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		
		// Create app manager
		appManager := app.NewAppManager(config.AppsDir)
		
		// Create the app
		if err := appManager.CreateApp(appName); err != nil {
			fmt.Printf("Error creating app: %v\n", err)
			return
		}
		
		// Update Caddyfile
		proxyManager := proxy.NewCaddyManager(config.ProxyDir)
		if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
			fmt.Printf("Error updating Caddyfile: %v\n", err)
			return
		}
		
		fmt.Printf("Application %s created successfully!\n", appName)
	},
}

var appsDeployCmd = &cobra.Command{
	Use:   "deploy [app-name]",
	Short: "Deploy an application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Deploying application: %s\n", appName)
		
		// Load config
		config, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		
		// Create app manager
		appManager := app.NewAppManager(config.AppsDir)
		
		// Load app configuration
		appConfig, err := appManager.LoadApp(appName)
		if err != nil {
			fmt.Printf("Error loading app config: %v\n", err)
			return
		}
		
		// Create docker manager
		dockerManager := docker.NewDockerManager(config.Registry.URL)
		
		// Generate docker-compose.yml
		appDir := filepath.Join(config.AppsDir, appName)
		if err := dockerManager.GenerateDockerCompose(appDir, appConfig.Services); err != nil {
			fmt.Printf("Error generating docker-compose: %v\n", err)
			return
		}
		
		// Deploy the application
		if err := dockerManager.DeployApp(appDir); err != nil {
			fmt.Printf("Error deploying app: %v\n", err)
			return
		}
		
		// Update Caddyfile
		proxyManager := proxy.NewCaddyManager(config.ProxyDir)
		if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
			fmt.Printf("Error updating Caddyfile: %v\n", err)
			return
		}
		
		fmt.Printf("Application %s deployed successfully!\n", appName)
	},
}

var appsDestroyCmd = &cobra.Command{
	Use:   "destroy [app-name]",
	Short: "Destroy an application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Destroying application: %s\n", appName)
		// TODO: Implement app destruction
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(appsCmd)
	
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsCreateCmd)
	appsCmd.AddCommand(appsDeployCmd)
	appsCmd.AddCommand(appsDestroyCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
