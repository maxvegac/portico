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
			cmd := exec.Command("docker", "compose", "-f", composeFile, "-p", appName, "ps", "-a", "--format", "json")
			cmd.Dir = appDir

			output, err := cmd.Output()
			if err != nil {
				// If no containers are running, output might be empty
				output = []byte{}
			}

			// Parse container information
			// Compose >= 2.21 prints one JSON object per line, older versions a JSON array
			var parsed []ContainerInfo
			trimmed := strings.TrimSpace(string(output))
			if strings.HasPrefix(trimmed, "[") {
				_ = json.Unmarshal([]byte(trimmed), &parsed)
			} else {
				for _, line := range strings.Split(trimmed, "\n") {
					var container ContainerInfo
					if line = strings.TrimSpace(line); line != "" && json.Unmarshal([]byte(line), &container) == nil {
						parsed = append(parsed, container)
					}
				}
			}

			// One entry per service (prefer a running replica), plus running counts
			containers := make(map[string]ContainerInfo)
			running := make(map[string]int)
			for _, container := range parsed {
				if strings.EqualFold(container.State, "running") {
					running[container.Service]++
				}
				if existing, ok := containers[container.Service]; !ok || !strings.EqualFold(existing.State, "running") {
					containers[container.Service] = container
				}
			}

			// Display header
			fmt.Printf("📦 Application: %s\n", appName)
			if a.Domain != "" {
				fmt.Printf("🌐 Domain: %s\n", a.Domain)
			}
			if a.Port > 0 {
				fmt.Printf("🔌 HTTP: %s -> %s:%d\n", appURL(a), a.HTTPServiceName(), a.Port)
			} else {
				fmt.Println("🔌 HTTP: disabled (background worker)")
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

				if a.Port > 0 && svc.Name == a.HTTPServiceName() {
					fmt.Printf("    HTTP:      port %d (via Caddy)\n", a.Port)
				}
				if len(svc.ExtraPorts) > 0 {
					fmt.Printf("    Ports:     %s\n", strings.Join(svc.ExtraPorts, ", "))
				}

				if svc.Replicas > 1 {
					statusText = fmt.Sprintf("%s (%d/%d replicas running)", statusText, running[svc.Name], svc.Replicas)
				}
				fmt.Printf("    Status:    %s", statusText)
				if exists && container.Status != "" && state != "running" {
					fmt.Printf(" (%s)", container.Status)
				}
				fmt.Println()

				if container.Name != "" {
					fmt.Printf("    Container: %s\n", container.Name)
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
