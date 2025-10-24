package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// getVersion gets the version from git tag or commit hash
func getVersion() string {
	// Try to get git tag first
	if tag, err := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(tag))
	}

	// If no tag, use commit hash
	if hash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(hash))
	}

	// Fallback to hardcoded version
	return "1.0.0"
}

var rootCmd = &cobra.Command{
	Use:   "portico",
	Short: "Portico - PaaS platform for managing applications",
	Long:  `Portico is a PaaS platform, using Caddy as reverse proxy and Docker Compose for applications.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Portico",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("Portico v%s\n", getVersion())
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
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Listing applications...")
		// TODO: Implement app listing
	},
}

var appsCreateCmd = &cobra.Command{
	Use:   "create [app-name]",
	Short: "Create a new application",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Creating application: %s\n", appName)

		// Load config
		config, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create app manager
		appManager := app.NewManager(config.AppsDir)

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
	Run: func(_ *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Deploying application: %s\n", appName)

		// Load config
		config, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create app manager
		appManager := app.NewManager(config.AppsDir)

		// Load app configuration
		appConfig, err := appManager.LoadApp(appName)
		if err != nil {
			fmt.Printf("Error loading app config: %v\n", err)
			return
		}

		// Create docker manager
		dockerManager := docker.NewManager(config.Registry.URL)

		// Generate docker-compose.yml
		appDir := filepath.Join(config.AppsDir, appName)

		// Convert app.Service to docker.Service
		var dockerServices []docker.Service
		for _, service := range appConfig.Services {
			dockerServices = append(dockerServices, docker.Service{
				Name:        service.Name,
				Image:       service.Image,
				Port:        service.Port,
				Environment: service.Environment,
				Volumes:     service.Volumes,
				Secrets:     service.Secrets,
				DependsOn:   service.DependsOn,
			})
		}

		if err := dockerManager.GenerateDockerCompose(appDir, dockerServices); err != nil {
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
	Run: func(_ *cobra.Command, args []string) {
		appName := args[0]
		fmt.Printf("Destroying application: %s\n", appName)
		// TODO: Implement app destruction
	},
}

func main() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(appsCmd)

	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsCreateCmd)
	appsCmd.AddCommand(appsDeployCmd)
	appsCmd.AddCommand(appsDestroyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
