package commands

import "github.com/spf13/cobra"

// NewAppsSetCmd is kept for backwards compatibility but is now empty
// All functionality has been moved to other commands
func NewAppsSetCmd() *cobra.Command {
	// This command is deprecated but kept to avoid breaking existing scripts
	return nil
}
