package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
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

			// Create the app
			if err := appManager.CreateApp(appName, port); err != nil {
				fmt.Printf("Error creating app: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(config.ProxyDir, config.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("Application %s created successfully!\n", appName)
		},
	}
}
