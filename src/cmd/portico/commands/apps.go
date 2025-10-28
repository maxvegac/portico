package commands

import (
	"github.com/spf13/cobra"
)

// NewAppsCmd creates the apps command
func NewAppsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apps",
		Short: "Manage applications",
		Long:  `Manage applications deployed on Portico platform.`,
	}
}
