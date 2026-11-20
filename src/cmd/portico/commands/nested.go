package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Several command groups take positional arguments *before* their subcommand,
// e.g. `portico ports <app> [service] add 8080 3000` or `portico set <app> http on`.
// Cobra cannot route that shape by itself, so the parent command gets a Run
// function (newNestedRun) that finds the subcommand in os.Args and invokes it,
// and the subcommands read the leading positionals with nestedPositionals.

// subcommandNames returns the names and aliases of the children of cmd
func subcommandNames(cmd *cobra.Command) map[string]bool {
	names := make(map[string]bool)
	for _, child := range cmd.Commands() {
		names[child.Name()] = true
		for _, alias := range child.Aliases {
			names[alias] = true
		}
	}
	return names
}

// isFlag reports whether a command line token is a flag
func isFlag(arg string) bool {
	return strings.HasPrefix(arg, "-") && arg != "-"
}

// nestedPositionalsFrom returns the non-flag arguments between the first occurrence
// of keyword and the first token that is a known subcommand.
func nestedPositionalsFrom(args []string, keyword string, subcommands map[string]bool) []string {
	start := -1
	for i, arg := range args {
		if arg == keyword {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	var positionals []string
	for _, arg := range args[start:] {
		if subcommands[arg] {
			break
		}
		if isFlag(arg) {
			continue
		}
		positionals = append(positionals, arg)
	}
	return positionals
}

// nestedPositionals returns the positionals given before the subcommand of
// the group that cmd belongs to (cmd is the subcommand being executed).
func nestedPositionals(cmd *cobra.Command) []string {
	parent := cmd.Parent()
	if parent == nil {
		return nil
	}
	return nestedPositionalsFrom(os.Args[1:], parent.Name(), subcommandNames(parent))
}

// nestedAppAndService returns the app name and the optional service name given
// before the subcommand: `<group> <app> [service] <subcommand> ...`
func nestedAppAndService(cmd *cobra.Command) (appName, serviceName string) {
	positionals := nestedPositionals(cmd)
	if len(positionals) > 0 {
		appName = positionals[0]
	}
	if len(positionals) > 1 {
		serviceName = positionals[1]
	}
	return appName, serviceName
}

// findSubcommand returns the child of parent named (or aliased) name
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name || child.HasAlias(name) {
			return child
		}
	}
	return nil
}

// runSubcommand parses flags for sub and runs it with the remaining arguments
func runSubcommand(sub *cobra.Command, args []string) {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			_ = sub.Help()
			return
		}
	}

	nonFlagArgs := args
	if !sub.DisableFlagParsing {
		if err := sub.ParseFlags(args); err != nil {
			fmt.Printf("Error parsing flags: %v\n", err)
			_ = sub.Help()
			return
		}
		nonFlagArgs = sub.Flags().Args()
	}

	if sub.Args != nil {
		if err := sub.Args(sub, nonFlagArgs); err != nil {
			fmt.Printf("Error: %v\n", err)
			_ = sub.Usage()
			return
		}
	}

	switch {
	case sub.Run != nil:
		sub.Run(sub, nonFlagArgs)
	case sub.RunE != nil:
		if err := sub.RunE(sub, nonFlagArgs); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	default:
		_ = sub.Help()
	}
}

// newNestedRun returns a Run function for a group command whose subcommand comes
// after one or more positional arguments.
func newNestedRun(keyword string) func(*cobra.Command, []string) {
	return func(parent *cobra.Command, _ []string) {
		args := os.Args[1:]
		start := -1
		for i, arg := range args {
			if arg == keyword {
				start = i + 1
				break
			}
		}
		if start < 0 {
			_ = parent.Help()
			return
		}

		subs := subcommandNames(parent)
		for i := start; i < len(args); i++ {
			if subs[args[i]] {
				runSubcommand(findSubcommand(parent, args[i]), args[i+1:])
				return
			}
		}

		_ = parent.Help()
	}
}
