package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHListCmd lists SSH public keys
func NewSSHListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SSH public keys",
		Long:  "List all SSH public keys that have access to git push deployment.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			porticoHome := cfg.PorticoHome
			authorizedKeysPath := filepath.Join(porticoHome, ".ssh", "authorized_keys")

			file, err := os.Open(authorizedKeysPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No SSH keys configured")
					return
				}
				fmt.Printf("Error reading authorized_keys: %v\n", err)
				return
			}
			defer func() {
				_ = file.Close()
			}()

			scanner := bufio.NewScanner(file)
			lineNum := 0
			hasKeys := false

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				hasKeys = true
				lineNum++

				// Extract key parts
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					// Has algorithm, key, and comment
					fmt.Printf("%d. %s %s\n   Comment: %s\n", lineNum, parts[0], parts[1][:20]+"...", strings.Join(parts[2:], " "))
				} else if len(parts) >= 2 {
					// Has algorithm and key, no comment
					fmt.Printf("%d. %s %s\n   Comment: (none)\n", lineNum, parts[0], parts[1][:20]+"...")
				}
			}

			if !hasKeys {
				fmt.Println("No SSH keys configured")
			}
		},
	}

	return cmd
}
