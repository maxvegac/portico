package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHRemoveCmd removes an SSH public key
func NewSSHRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [index|key]",
		Short: "Remove an SSH public key",
		Long: `Remove an SSH public key by index (from 'portico ssh list') or by key content.

Examples:
  # Remove by index (from list command)
  portico ssh remove 1

  # Remove by key content (partial match)
  portico ssh remove "ssh-rsa AAAAB3..."`,
		Args: cobra.ExactArgs(1),
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

			// Read all keys
			var lines []string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				return
			}

			// Determine if argument is index or key content
			arg := strings.TrimSpace(args[0])
			index, err := strconv.Atoi(arg)

			var keysToRemove []int
			if err == nil {
				// It's an index
				if index < 1 || index > len(lines) {
					fmt.Printf("Error: Invalid index %d. Use 'portico ssh list' to see available keys\n", index)
					return
				}

				// Count non-empty, non-comment lines
				validLineNum := 0
				for i, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						validLineNum++
						if validLineNum == index {
							keysToRemove = append(keysToRemove, i)
							break
						}
					}
				}
			} else {
				// It's a key content (partial match)
				for i, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, arg) {
						keysToRemove = append(keysToRemove, i)
					}
				}

				if len(keysToRemove) == 0 {
					fmt.Printf("No keys found matching: %s\n", arg)
					return
				}
			}

			// Remove keys (in reverse order to maintain indices)
			for i := len(keysToRemove) - 1; i >= 0; i-- {
				idx := keysToRemove[i]
				lines = append(lines[:idx], lines[idx+1:]...)
			}

			// Write back to file
			if err := os.WriteFile(authorizedKeysPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
				fmt.Printf("Error writing authorized_keys: %v\n", err)
				return
			}

			fmt.Printf("✅ Removed %d SSH key(s)\n", len(keysToRemove))
		},
	}

	return cmd
}
