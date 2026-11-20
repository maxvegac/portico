package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the Portico version, injected at build time via main.version
// (go build -ldflags "-X main.version=v1.2.3"). "dev" for local builds.
var Version = "dev"

// getVersion returns the version of this binary.
// It used to run `git describe` in the current directory, which reported the
// version of whatever repository the user happened to be in.
func getVersion() string {
	return Version
}

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Portico",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("Portico %s\n", getVersion())
		},
	}
}
