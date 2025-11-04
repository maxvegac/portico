package commands

import (
	"github.com/spf13/cobra"
)

// NewSSHCmd creates the ssh command for managing SSH keys
func NewSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage SSH keys for git deployment",
		Long:  "Manage SSH public keys for git push deployment. These keys allow users to push code to Portico repositories.",
	}

	// Add subcommands
	cmd.AddCommand(NewSSHAddCmd())
	cmd.AddCommand(NewSSHListCmd())
	cmd.AddCommand(NewSSHRemoveCmd())

	return cmd
}
