package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// addonActions are the actions that follow an app/instance name:
//
//	addons <app> add <type>          addons <instance> up|down|delete
//	addons <app> link <instance>     addons <instance> database create|delete|list [db]
var addonActions = map[string]bool{
	"add":      true,
	"link":     true,
	"up":       true,
	"down":     true,
	"delete":   true,
	"database": true,
}

// NewAddonsCmd creates the addons command
func NewAddonsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addons",
		Short: "Manage addons (databases, cache, tools)",
		Long: `Manage addons such as databases, cache stores, and administration tools.

Examples:
  portico addons list
  portico addons create my-postgres --type postgresql --version 18
  portico addons my-postgres up
  portico addons my-postgres database create mydb
  portico addons my-app link my-postgres --database mydb
  portico addons my-app add redis --version 8`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true, // flags belong to the subcommands (e.g. link --database)
		Run:                runAddonsNested,
	}

	// Commands that take the form `addons <command> ...`
	cmd.AddCommand(NewAddonsListCmd())
	cmd.AddCommand(NewAddonsInstancesCmd())
	cmd.AddCommand(NewAddonCreateCmd())

	// Commands that take the form `addons <name> <action> ...` (dispatched by runAddonsNested)
	cmd.AddCommand(NewAddonsInstanceCmd())
	databaseCmd := NewAddonDatabaseCmd()
	databaseCmd.AddCommand(NewAddonDatabaseCreateCmd())
	databaseCmd.AddCommand(NewAddonDatabaseDeleteCmd())
	databaseCmd.AddCommand(NewAddonDatabaseListCmd())
	cmd.AddCommand(databaseCmd)
	cmd.AddCommand(NewAddonAddCmd())
	cmd.AddCommand(NewAddonLinkCmd())

	return cmd
}

// runAddonsNested dispatches `addons <name> <action> ...`
func runAddonsNested(parent *cobra.Command, _ []string) {
	args := os.Args[1:]
	start := -1
	for i, arg := range args {
		if arg == "addons" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		_ = parent.Help()
		return
	}

	for i := start; i < len(args); i++ {
		action := args[i]
		if !addonActions[action] {
			continue
		}
		rest := args[i+1:]

		var target *cobra.Command
		switch action {
		case "up", "down", "delete":
			if instanceCmd := findSubcommand(parent, "[instance-name]"); instanceCmd != nil {
				target = findSubcommand(instanceCmd, action)
			}
		case "database":
			databaseCmd := findSubcommand(parent, "database")
			if databaseCmd == nil {
				break
			}
			for j, arg := range rest {
				if sub := findSubcommand(databaseCmd, arg); sub != nil && !isFlag(arg) {
					runSubcommand(sub, rest[j+1:])
					return
				}
			}
			_ = databaseCmd.Help()
			return
		default:
			target = findSubcommand(parent, action)
		}

		if target == nil {
			break
		}
		runSubcommand(target, rest)
		return
	}

	fmt.Println("Error: unknown addons command")
	_ = parent.Help()
}
