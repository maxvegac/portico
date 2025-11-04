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

// NewStorageDeleteCmd removes a volume mount from a service
func NewStorageDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [host-path] [container-path]",
		Short: "Remove a volume mount",
		Long:  "Remove a volume mount from a service in the given app. If the app has only one service, service-name is optional.\n\nArguments:\n  - host-path: Path on the host\n  - container-path: Path inside the container\n\nExample:\n  portico storage my-app delete /data/my-app/data /app/data",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (storage)
			appName, err := getAppNameFromStorageArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico storage [app-name] delete [host-path] [container-path]")
				return
			}

			hostPath := strings.TrimSpace(args[0])
			containerPath := strings.TrimSpace(args[1])

			if hostPath == "" || containerPath == "" {
				fmt.Println("Invalid paths")
				return
			}

			volumeMount := fmt.Sprintf("%s:%s", hostPath, containerPath)

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

			// Find service and remove volume
			found := false
			removed := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					filtered := make([]string, 0, len(a.Services[i].Volumes))
					for _, v := range a.Services[i].Volumes {
						if v == volumeMount {
							removed = true
							continue
						}
						filtered = append(filtered, v)
					}
					a.Services[i].Volumes = filtered
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}
			if !removed {
				fmt.Printf("Volume mount %s not found for service %s in %s\n", volumeMount, serviceName, appName)
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
				replicas := s.Replicas
				if replicas == 0 {
					replicas = 1 // Default to 1 if not specified
				}
				dockerServices = append(dockerServices, docker.Service{
					Name:        s.Name,
					Image:       s.Image,
					Port:        s.Port,
					ExtraPorts:  s.ExtraPorts,
					Environment: s.Environment,
					Volumes:     s.Volumes,
					Secrets:     s.Secrets,
					DependsOn:   s.DependsOn,
					Replicas:    replicas,
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
			if err := dm.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			fmt.Printf("Removed volume mount %s from service %s in %s\n", volumeMount, serviceName, appName)
		},
	}

	cmd.Flags().String("name", "", "service name (required if app has multiple services)")
	return cmd
}
