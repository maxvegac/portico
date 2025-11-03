package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewStorageAddCmd adds a volume mount to a service
func NewStorageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [host-path] [container-path]",
		Short: "Add a volume mount",
		Long:  "Add a volume mount to a service in the given app. If the app has only one service, service-name is optional.\n\nArguments:\n  - host-path: Path on the host (absolute or relative to app directory)\n  - container-path: Path inside the container\n\nExamples:\n  portico storage my-app add /data/my-app/data /app/data\n    Mounts host /data/my-app/data to container /app/data (uses default service if only one exists)\n\n  portico storage my-app add ./data /app/data\n    Mounts ./data (relative to app directory) to container /app/data",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (storage)
			appName, err := getAppNameFromStorageArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico storage [app-name] add [host-path] [container-path]")
				return
			}

			hostPath := strings.TrimSpace(args[0])
			containerPath := strings.TrimSpace(args[1])

			if hostPath == "" || containerPath == "" {
				fmt.Println("Invalid paths")
				return
			}

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

			// Determine service name - if only one service, use it; otherwise require service-name flag
			var serviceName string
			if len(a.Services) == 1 {
				serviceName = a.Services[0].Name
			} else {
				// Try to get from flag or require it
				serviceNameFlag, _ := cmd.Flags().GetString("name")
				if serviceNameFlag == "" {
					var serviceNames []string
					for _, s := range a.Services {
						serviceNames = append(serviceNames, s.Name)
					}
					fmt.Printf("Error: app %s has %d services. Please specify service name with --name flag\n", appName, len(a.Services))
					fmt.Printf("Available services: %v\n", serviceNames)
					return
				}
				serviceName = serviceNameFlag
			}

			// Find service
			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true

					// Check if volume already exists
					volumeMount := fmt.Sprintf("%s:%s", hostPath, containerPath)
					for _, v := range a.Services[i].Volumes {
						if v == volumeMount {
							fmt.Printf("Volume mount %s already exists for service %s in %s\n", volumeMount, serviceName, appName)
							return
						}
					}

					// Add volume
					a.Services[i].Volumes = append(a.Services[i].Volumes, volumeMount)
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			var dockerServices []docker.Service
			for _, s := range a.Services {
				dockerServices = append(dockerServices, docker.Service{
					Name:        s.Name,
					Image:       s.Image,
					Port:        s.Port,
					ExtraPorts:  s.ExtraPorts,
					Environment: s.Environment,
					Volumes:     s.Volumes,
					Secrets:     s.Secrets,
					DependsOn:   s.DependsOn,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: a.Domain,
				Port:   a.Port,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}
			if err := dm.DeployApp(appDir); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			fmt.Printf("Added volume mount: %s -> %s for service %s in %s\n", hostPath, containerPath, serviceName, appName)
		},
	}

	cmd.Flags().String("name", "", "service name (required if app has multiple services)")
	return cmd
}
