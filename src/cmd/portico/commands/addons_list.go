package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonsListCmd lists available addons and their versions
func NewAddonsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [addon-type]",
		Short: "List available addons or versions",
		Long:  "List all available addon types, or list versions for a specific addon type.\n\nExamples:\n  portico addons list\n  portico addons list postgresql",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))

			// Available addon types
			addonTypes := []string{"postgresql", "mariadb", "mysql", "mongodb", "redis", "valkey"}

			if len(args) == 0 {
				// List all addon types
				fmt.Println("Available addon types:")
				fmt.Println()

				for _, addonType := range addonTypes {
					def, err := am.LoadDefinition(addonType)
					if err != nil {
						fmt.Printf("  %s - (definition not found)\n", addonType)
						continue
					}

					versions := def.GetAvailableVersions()
					fmt.Printf("  %s - %s\n", addonType, def.Description)
					fmt.Printf("    Type: %s, Mode: %s\n", def.Type, def.ServiceMode)
					if len(versions) > 0 {
						fmt.Printf("    Versions: %v\n", versions)
					}
					fmt.Println()
				}
			} else {
				// List versions for specific addon type
				addonType := args[0]
				def, err := am.LoadDefinition(addonType)
				if err != nil {
					fmt.Printf("Error loading addon definition: %v\n", err)
					return
				}

				versions := def.GetAvailableVersions()
				fmt.Printf("Addon: %s\n", addonType)
				fmt.Printf("Description: %s\n", def.Description)
				fmt.Printf("Type: %s\n", def.Type)
				fmt.Printf("Mode: %s\n", def.ServiceMode)
				fmt.Printf("Default Port: %d\n", def.DefaultPort)
				fmt.Println()
				fmt.Printf("Available versions:\n")
				for _, version := range versions {
					versionConfig, err := def.GetVersionConfig(version)
					if err == nil {
						fmt.Printf("  %s - %s\n", version, versionConfig.Image)
					} else {
						fmt.Printf("  %s\n", version)
					}
				}
			}
		},
	}

	return cmd
}
