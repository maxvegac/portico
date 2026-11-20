package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewDomainsAddCmd adds a domain to an application
func NewDomainsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [domain]",
		Short: "Add domain to application",
		Long:  "Add a domain to the application, update docker-compose.yml, regenerate the app Caddyfile, and refresh the reverse proxy.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (domains)
			appName, err := getAppNameFromDomainsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico domains [app-name] add [domain]")
				return
			}
			domain := args[0]

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

			domain = strings.ToLower(strings.TrimSpace(domain))
			if err := validateDomain(domain); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			a.Domain = domain
			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}
			if err := syncProxy(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("Domain %s added to %s\n", domain, appName)
		},
	}
}
