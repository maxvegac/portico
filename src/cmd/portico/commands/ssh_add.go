package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHAddCmd adds an SSH public key
func NewSSHAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [key-or-file] [key-name]",
		Short: "Add an SSH public key",
		Long: `Add an SSH public key to allow git push deployment.

If the first argument is a file path that exists, it will be read as a key file.
Otherwise, it will be treated as the key content itself.
If key-name is not provided, a default name will be generated.

Examples:
  # Add key from file with custom name
  portico ssh add ~/.ssh/id_rsa.pub "ci-deployment"

  # Add key from file with default name
  portico ssh add ~/.ssh/id_rsa.pub

  # Add key directly with custom name
  portico ssh add "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAA..." "my-key"

  # Add key directly with default name
  portico ssh add "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAA..."

  # Add key from stdin
  cat ~/.ssh/id_rsa.pub | portico ssh add - "my-key"`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			porticoHome := cfg.PorticoHome
			sshDir := filepath.Join(porticoHome, ".ssh")
			authorizedKeysPath := filepath.Join(sshDir, "authorized_keys")

			var keyContent string
			var keyName string

			// Get key name (second argument or default)
			if len(args) >= 2 {
				keyName = strings.TrimSpace(args[1])
			}
			if keyName == "" {
				// Generate default name based on timestamp
				keyName = fmt.Sprintf("key-%d", time.Now().Unix())
			}

			// Get key content
			firstArg := strings.TrimSpace(args[0])

			// Check if first argument is a file (or "-" for stdin)
			if firstArg == "-" {
				// Read from stdin
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					keyContent = strings.TrimSpace(scanner.Text())
				} else {
					fmt.Println("Error: No key provided from stdin")
					return
				}
			} else if _, err := os.Stat(firstArg); err == nil {
				// File exists, read from file
				data, err := os.ReadFile(firstArg)
				if err != nil {
					fmt.Printf("Error reading key file: %v\n", err)
					return
				}
				keyContent = strings.TrimSpace(string(data))
			} else {
				// Treat as key content directly
				keyContent = firstArg
			}

			if keyContent == "" {
				fmt.Println("Error: Empty key provided")
				return
			}

			// Validate key format (basic check)
			parts := strings.Fields(keyContent)
			if len(parts) < 2 {
				fmt.Println("Error: Invalid SSH key format. Expected format: 'algorithm key-data [comment]'")
				return
			}

			// Replace or add comment with key name
			keyParts := strings.Fields(keyContent)
			if len(keyParts) >= 2 {
				// Keep algorithm and key, replace comment with key name
				keyContent = fmt.Sprintf("%s %s %s", keyParts[0], keyParts[1], keyName)
			}

			// Ensure .ssh directory exists
			if err := os.MkdirAll(sshDir, 0o700); err != nil {
				fmt.Printf("Error creating .ssh directory: %v\n", err)
				return
			}

			// Read existing keys to check for duplicates
			existingKeys := make(map[string]bool)
			if data, err := os.ReadFile(authorizedKeysPath); err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						// Extract key part (algorithm + key data)
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							keyPart := fmt.Sprintf("%s %s", parts[0], parts[1])
							existingKeys[keyPart] = true
						}
					}
				}
			}

			// Check if key already exists
			keyParts = strings.Fields(keyContent)
			if len(keyParts) >= 2 {
				keyPart := fmt.Sprintf("%s %s", keyParts[0], keyParts[1])
				if existingKeys[keyPart] {
					fmt.Println("⚠️  This SSH key already exists")
					return
				}
			}

			// Append key to authorized_keys
			file, err := os.OpenFile(authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				fmt.Printf("Error opening authorized_keys: %v\n", err)
				return
			}
			defer func() {
				_ = file.Close()
			}()

			if _, err := file.WriteString(keyContent + "\n"); err != nil {
				fmt.Printf("Error writing key: %v\n", err)
				return
			}

			fmt.Printf("✅ SSH key added successfully (name: %s)\n", keyName)
		},
	}

	return cmd
}
