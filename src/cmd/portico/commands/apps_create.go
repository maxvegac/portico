package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewAppsCreateCmd creates the apps create command
func NewAppsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [app-name]",
		Short: "Create a new application",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			fmt.Printf("Creating application: %s\n", appName)

			// Ask for internal port
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Internal HTTP port (default: 8080): ")
			portStr, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				return
			}
			portStr = strings.TrimSpace(portStr)

			port := 8080
			if portStr != "" {
				port, err = strconv.Atoi(portStr)
				if err != nil || port <= 0 || port > 65535 {
					fmt.Println("Invalid port, using default 8080")
					port = 8080
				}
			}

			// Load config
			config, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(config.AppsDir, config.TemplatesDir)

			// Create app directories and secrets
			if err := appManager.CreateAppDirectories(appName); err != nil {
				fmt.Printf("Error creating app directories: %v\n", err)
				return
			}

			// Generate docker-compose.yml directly (no app.yml needed)
			dockerManager := docker.NewManager(config.Registry.URL)
			appDir := filepath.Join(config.AppsDir, appName)

			// Create default service
			dockerServices := []docker.Service{
				{
					Name:  "web",
					Image: "node:22-alpine",
					Port:  3000,
					Environment: map[string]string{
						"NODE_ENV": "production",
						"PORT":     "3000",
					},
				},
			}

			metadata := &docker.PorticoMetadata{
				Domain: fmt.Sprintf("%s.localhost", appName),
				Port:   port,
			}

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Create default Caddyfile
			if err := appManager.CreateDefaultCaddyfile(appName); err != nil {
				fmt.Printf("Error creating Caddyfile: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(config.ProxyDir, config.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			// Create git repository for the app
			porticoHome := filepath.Dir(config.AppsDir)
			reposDir := filepath.Join(porticoHome, "repos")

			if err := os.MkdirAll(reposDir, 0o755); err == nil {
				repoDir := filepath.Join(reposDir, appName+".git")

				// Create bare repository
				cmd := exec.Command("git", "init", "--bare", repoDir)
				if err := cmd.Run(); err == nil {
					// Create post-receive hook that calls portico git-receive
					postReceiveDst := filepath.Join(repoDir, "hooks", "post-receive")
					hookContent := "#!/bin/sh\nexec portico git-receive\n"

					if err := os.WriteFile(postReceiveDst, []byte(hookContent), 0o755); err == nil {
						hostname, _ := os.Hostname()
						if hostname == "" {
							hostname = "portico-server"
						}
						fmt.Printf("Git repository created: portico@%s:%s.git\n", hostname, appName)
					}
				}
			}

			fmt.Printf("Application %s created successfully!\n", appName)
		},
	}
}
