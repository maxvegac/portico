package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// ContainerInfo represents container information from docker compose ps
type ContainerInfo struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
}

// NewAppsStatusCmd creates the apps status command
func NewAppsStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [app-name]",
		Short: "Show application services and their status",
		Long:  "Display the status of all services in an application, including running containers and their states.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			composeFile := filepath.Join(appDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("docker-compose.yml not found for app %s\n", appName)
				return
			}

			// Get container status using docker compose ps
			cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--format", "json")
			cmd.Dir = appDir

			output, err := cmd.Output()
			if err != nil {
				// If no containers are running, output might be empty
				output = []byte{}
			}

			// Parse container information
			containers := make(map[string]ContainerInfo)
			if len(output) > 0 {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					var container ContainerInfo
					if err := json.Unmarshal([]byte(line), &container); err == nil {
						containers[container.Service] = container
					}
				}
			}

			// Display header
			fmt.Printf("📦 Application: %s\n", appName)
			if a.Domain != "" {
				fmt.Printf("🌐 Domain: %s\n", a.Domain)
			}
			if a.Port > 0 {
				fmt.Printf("🔌 Port: %d\n", a.Port)
			}
			fmt.Println()

			if len(a.Services) == 0 {
				fmt.Println("⚠️  No services defined")
				return
			}

			// Display services table
			fmt.Println("Services:")
			fmt.Println(strings.Repeat("─", 80))

			for i, svc := range a.Services {
				if i > 0 {
					fmt.Println()
				}

				container, exists := containers[svc.Name]
				statusIcon := "○"
				statusText := "Not running"
				state := ""

				if exists {
					state = container.State
					switch state {
					case "running":
						statusIcon = "✓"
						statusText = "Running"
					case "exited":
						statusIcon = "✗"
						statusText = "Stopped"
					case "restarting":
						statusIcon = "↻"
						statusText = "Restarting"
					default:
						// Capitalize first letter
						stateLower := strings.ToLower(state)
						if len(stateLower) > 0 {
							stateCapitalized := strings.ToUpper(stateLower[:1]) + stateLower[1:]
							statusText = stateCapitalized
						} else {
							statusText = state
						}
					}
				}

				fmt.Printf("  %s %s\n", statusIcon, svc.Name)
				fmt.Printf("    Image:     %s\n", svc.Image)

				if svc.Port > 0 {
					fmt.Printf("    Port:      %d\n", svc.Port)
				}

				fmt.Printf("    Status:    %s", statusText)
				if exists && container.Status != "" && state != "running" {
					fmt.Printf(" (%s)", container.Status)
				}
				fmt.Println()

				if container.Name != "" {
					fmt.Printf("    Container: %s\n", container.Name)
				}

				// Show extra ports if any
				if len(svc.ExtraPorts) > 0 {
					fmt.Printf("    Ports:     %s\n", strings.Join(svc.ExtraPorts, ", "))
				}
			}

			fmt.Println(strings.Repeat("─", 80))

			// Summary
			runningCount := 0
			for _, svc := range a.Services {
				if container, exists := containers[svc.Name]; exists && container.State == "running" {
					runningCount++
				}
			}

			fmt.Printf("\nSummary: %d/%d services running\n", runningCount, len(a.Services))
		},
	}

	return cmd
}
