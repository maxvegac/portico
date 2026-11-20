package commands

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonsInstancesCmd lists addon instances
func NewAddonsInstancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "List addon instances",
		Long:  "List all created addon instances with their configuration.",
		Args:  cobra.ExactArgs(0),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			config, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			if len(config.Instances) == 0 {
				fmt.Println("No addon instances created yet.")
				return
			}

			fmt.Println("Addon instances:")
			fmt.Println()

			names := make([]string, 0, len(config.Instances))
			for name := range config.Instances {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				instance := config.Instances[name]
				fmt.Printf("  %s\n", name)
				fmt.Printf("    Type: %s\n", instance.Type)
				fmt.Printf("    Version: %s\n", instance.Version)
				fmt.Printf("    Mode: %s\n", instance.Mode)
				fmt.Printf("    Port: %d (127.0.0.1)\n", instance.Port)
				fmt.Printf("    Host: %s\n", addonHost(filepath.Join(cfg.AddonsDir, "instances", name), instance))
				if instance.Target != "" {
					fmt.Printf("    Target: %s\n", instance.Target)
				}
				if instance.Domain != "" {
					fmt.Printf("    URL: https://%s\n", instance.Domain)
				}
				if instance.Mode == "dedicated" {
					fmt.Printf("    App: %s\n", instance.App)
				} else if len(instance.Apps) > 0 {
					fmt.Printf("    Apps: %v\n", instance.Apps)
				}
				fmt.Println()
			}
		},
	}

	return cmd
}
