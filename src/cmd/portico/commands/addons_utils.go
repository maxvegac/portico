package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// addonsPositionals returns the non-flag arguments between "addons" and the action
func addonsPositionals() []string {
	return nestedPositionalsFrom(os.Args[1:], "addons", addonActions)
}

// getAppNameFromAddonsArgs extracts app-name from `addons <app> add|link ...`
func getAppNameFromAddonsArgs(_ *cobra.Command) (string, error) {
	positionals := addonsPositionals()
	if len(positionals) == 0 {
		return "", fmt.Errorf("app name not found")
	}
	return positionals[0], nil
}

// getInstanceNameFromAddonsArgs extracts the instance name from `addons <instance> up|down|delete|database ...`
func getInstanceNameFromAddonsArgs(_ *cobra.Command) (string, error) {
	positionals := addonsPositionals()
	if len(positionals) == 0 {
		return "", fmt.Errorf("instance name not found")
	}
	return positionals[0], nil
}
