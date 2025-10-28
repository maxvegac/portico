package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// getVersion gets the version from git tag or commit hash
func getVersion() string {
	// Try to get git tag first
	if tag, err := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(tag))
	}

	// If no tag, use commit hash
	if hash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(hash))
	}

	// Fallback to hardcoded version
	return "1.0.0"
}

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Portico",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("Portico v%s\n", getVersion())
		},
	}
}
