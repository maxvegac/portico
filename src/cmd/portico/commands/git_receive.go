package commands

import (
	"bufio"
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

// NewGitReceiveCmd handles git post-receive hook
func NewGitReceiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "git-receive",
		Short:  "Handle git post-receive hook (internal use)",
		Hidden: true, // Hide from help since it's only used by git hooks
		Args:   cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			// Get app name from current directory (git repo)
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current directory: %v\n", err)
				os.Exit(1)
			}

			// Extract app name from repo directory (e.g., /home/portico/repos/my-app.git -> my-app)
			repoName := filepath.Base(cwd)
			appName := strings.TrimSuffix(repoName, ".git")

			if appName == "" {
				fmt.Println("Error: could not determine app name from repository directory")
				os.Exit(1)
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				os.Exit(1)
			}

			// Create temporary directory in /home/portico/.tmp
			tmpDir := filepath.Join(filepath.Dir(cfg.AppsDir), ".tmp", fmt.Sprintf("%s-%d", appName, os.Getpid()))
			if err := os.MkdirAll(tmpDir, 0o755); err != nil {
				fmt.Printf("Error creating temporary directory: %v\n", err)
				os.Exit(1)
			}
			defer func() {
				if err := os.RemoveAll(tmpDir); err != nil {
					fmt.Printf("Warning: could not remove temporary directory: %v\n", err)
				}
			}()

			// Read git push information from stdin
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					// Extract refname (branch name)
					refname := parts[2]
					// Checkout the code to temporary directory
					cmd := exec.Command("git", "--work-tree", tmpDir, "--git-dir", cwd, "checkout", "-f", refname)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						fmt.Printf("Error checking out code: %v\n", err)
						os.Exit(1)
					}
					break // Only process first ref
				}
			}

			if err := scanner.Err(); err != nil {
				fmt.Printf("Error reading stdin: %v\n", err)
				os.Exit(1)
			}

			// Change to temporary directory
			oldCwd, _ := os.Getwd()
			if err := os.Chdir(tmpDir); err != nil {
				fmt.Printf("Error changing to temporary directory: %v\n", err)
				os.Exit(1)
			}
			defer func() {
				_ = os.Chdir(oldCwd)
			}()

			// Deploy using Portico
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				// App doesn't exist, create it
				fmt.Printf("App %s not found. Creating app...\n", appName)
				if err := appManager.CreateAppDirectories(appName); err != nil {
					fmt.Printf("Error creating app directories: %v\n", err)
					os.Exit(1)
				}
				appConfig, err = appManager.LoadApp(appName)
				if err != nil {
					fmt.Printf("Error loading newly created app: %v\n", err)
					os.Exit(1)
				}
			}

			// Check for Dockerfile
			dockerfile := "Dockerfile"
			if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
				fmt.Printf("Error: Dockerfile not found in repository\n")
				os.Exit(1)
			}

			// Generate image name
			imageName := fmt.Sprintf("portico-%s:latest", appName)

			// Build Docker image
			fmt.Printf("Building Docker image: %s\n", imageName)
			buildCmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerfile, ".")
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			if err := buildCmd.Run(); err != nil {
				fmt.Printf("Error building Docker image: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Docker image built successfully: %s\n", imageName)

			// Update app config with new image
			updated := false
			for i := range appConfig.Services {
				if appConfig.Services[i].Name == "web" || len(appConfig.Services) == 1 {
					appConfig.Services[i].Image = imageName
					updated = true
					break
				}
			}

			if !updated && len(appConfig.Services) > 0 {
				// Update first service if no "web" service found
				appConfig.Services[0].Image = imageName
			}

			// Save app configuration
			if err := appManager.SaveApp(appConfig); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				os.Exit(1)
			}

			// Generate docker-compose.yml
			dockerManager := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			var dockerServices []docker.Service
			for _, svc := range appConfig.Services {
				replicas := svc.Replicas
				if replicas == 0 {
					replicas = 1
				}
				dockerServices = append(dockerServices, docker.Service{
					Name:        svc.Name,
					Image:       svc.Image,
					Port:        svc.Port,
					ExtraPorts:  svc.ExtraPorts,
					Environment: svc.Environment,
					Volumes:     svc.Volumes,
					Secrets:     svc.Secrets,
					DependsOn:   svc.DependsOn,
					Replicas:    replicas,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: appConfig.Domain,
				Port:   appConfig.Port,
			}

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				os.Exit(1)
			}

			// Deploy the application
			if err := dockerManager.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				os.Exit(1)
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Application %s deployed successfully!\n", appName)
		},
	}

	return cmd
}
