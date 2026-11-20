package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAppsCreateCmd creates the apps create command
func NewAppsCreateCmd() *cobra.Command {
	var withService string
	var image string
	var noHTTPPort bool
	var servicePort int

	cmd := &cobra.Command{
		Use:   "create [app-name]",
		Short: "Create a new application",
		Long: `Create a new application with directory structure and git repository.
		
By default, this command only creates the necessary directories and git repository.
Services are created when you deploy an image using 'portico deploy' or 'portico service ... image'.

You can optionally create a service immediately using --with-service flag.

Examples:
  # Create app (no services created yet)
  portico create my-app

  # Create app with a web service
  portico create my-app --with-service web --image myregistry.com/my-app:v1.0.0

  # Create app with a background worker
  portico create my-app --with-service worker --image myregistry.com/worker:v1.0.0 --no-http-port

  # Deploy will create services automatically
  portico deploy my-app`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appName := args[0]
			if err := util.ValidateNewAppName(appName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			if withService != "" {
				if err := util.ValidateServiceName(withService); err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				if image == "" {
					fmt.Printf("Error: --image is required when using --with-service\n")
					return
				}
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			if appManager.Exists(appName) {
				fmt.Printf("Error: application %s already exists\n", appName)
				return
			}

			fmt.Printf("Creating application: %s\n", appName)

			// Internal port Caddy proxies to (0 = background worker, no HTTP)
			httpPort := app.DefaultHTTPPort
			if servicePort > 0 {
				httpPort = servicePort
			}
			if noHTTPPort {
				httpPort = 0
			}

			appConfig, err := createAppSkeleton(cfg, appManager, appName, httpPort)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Create git repository for the app (push-to-deploy)
			if err := createGitRepo(cfg.PorticoHome, appName); err != nil {
				fmt.Printf("Warning: could not create git repository: %v\n", err)
			} else {
				hostname, _ := os.Hostname()
				if hostname == "" {
					hostname = "portico-server"
				}
				fmt.Printf("Git repository created: portico@%s:%s.git\n", hostname, appName)
			}

			// If --with-service flag is provided, create the service and deploy it
			if withService != "" {
				appConfig.Services = []app.Service{{
					Name:        withService,
					Image:       image,
					Port:        httpPort,
					Environment: map[string]string{},
				}}
				if httpPort > 0 {
					appConfig.HttpService = withService
				}

				pullImage(image)
				if err := applyApp(cfg, appManager, appConfig); err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}

				fmt.Printf("✅ Application %s created successfully with service %s!\n", appName, withService)
				if url := appURL(appConfig); url != "" {
					fmt.Printf("URL: %s\n", url)
				}
				return
			}

			fmt.Printf("Application %s created successfully!\n", appName)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  To create a web service:\n")
			fmt.Printf("    portico service %s web image [your-image] --port [container-port]\n", appName)
			fmt.Println()
			fmt.Printf("  To deploy from source code (in a directory with a Dockerfile):\n")
			fmt.Printf("    portico deploy %s\n", appName)
			fmt.Println()
			fmt.Printf("  To create a background worker:\n")
			fmt.Printf("    portico service %s worker image [your-image] --no-http-port\n", appName)
		},
	}

	cmd.Flags().StringVar(&withService, "with-service", "", "Create a service immediately (requires --image)")
	cmd.Flags().StringVar(&image, "image", "", "Docker image for the service (required with --with-service)")
	cmd.Flags().BoolVar(&noHTTPPort, "no-http-port", false, "Create a background worker without HTTP port")
	cmd.Flags().IntVar(&servicePort, "port", 0, "Internal container port Caddy proxies to (default: 8080)")

	return cmd
}

// createGitRepo creates the bare repository used for git push deployments
func createGitRepo(porticoHome, appName string) error {
	reposDir := filepath.Join(porticoHome, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		return err
	}

	repoDir := filepath.Join(reposDir, appName+".git")
	if _, err := os.Stat(repoDir); err == nil {
		return nil // already exists
	}

	if output, err := exec.Command("git", "init", "--bare", repoDir).CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %v: %s", err, string(output))
	}

	// post-receive hook that calls portico git-receive.
	// Use the absolute path of this binary so the hook works even with a minimal PATH.
	porticoBin := "portico"
	if exe, err := os.Executable(); err == nil {
		porticoBin = exe
	}
	hookPath := filepath.Join(repoDir, "hooks", "post-receive")
	hookContent := fmt.Sprintf("#!/bin/sh\nexec %q git-receive\n", porticoBin)
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return err
	}

	_ = util.FixDirOwnership(repoDir)
	return nil
}
