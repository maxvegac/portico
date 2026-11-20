package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHListCmd lists SSH public keys
func NewSSHListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List SSH public keys",
		Long:  "List all SSH public keys that have access to git push deployment.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			lines, err := readAuthorizedKeys(authorizedKeysPath(cfg.PorticoHome))
			if err != nil {
				fmt.Printf("Error reading authorized_keys: %v\n", err)
				return
			}

			index := 0
			for _, line := range lines {
				key, err := parseAuthorizedKey(line)
				if err != nil {
					continue
				}
				index++

				name := key.Comment
				if name == "" {
					name = "(no name)"
				}
				access := "full shell"
				if key.Restricted() {
					access = "git only"
				} else if key.Options != "" {
					access = "custom options"
				}
				fmt.Printf("%d. %s\n   %s %s (%s)\n", index, name, key.Type, key.Fingerprint(), access)
			}

			if index == 0 {
				fmt.Println("No SSH keys configured")
			}
		},
	}
}
