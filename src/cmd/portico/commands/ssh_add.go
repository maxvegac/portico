package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHAddCmd adds an SSH public key
func NewSSHAddCmd() *cobra.Command {
	var admin bool

	cmd := &cobra.Command{
		Use:   "add [key-or-file] [key-name]",
		Short: "Add an SSH public key",
		Long: `Add an SSH public key to allow git push deployment.

If the first argument is a file path that exists, it will be read as a key file.
Use "-" to read the key from stdin. Otherwise it is treated as the key itself.
If key-name is not provided, a default name will be generated.

By default the key can only run git push/fetch against Portico repositories
(the portico user is in the docker group, so a shell would be root-equivalent).
Use --admin to grant full shell access.

Examples:
  # Add key from file with custom name
  portico ssh add ~/.ssh/id_ed25519.pub ci-deployment

  # Add key directly
  portico ssh add "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..." my-key

  # Add key from stdin
  cat ~/.ssh/id_ed25519.pub | portico ssh add - my-key

Then deploy with:
  git remote add portico portico@<server>:<app-name>.git
  git push portico main`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			keyName := fmt.Sprintf("key-%d", time.Now().Unix())
			if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
				keyName = strings.TrimSpace(args[1])
			}
			if !keyNameRe.MatchString(keyName) {
				fmt.Printf("Error: invalid key name %q (use letters, digits and . _ @ : + -, max 64 chars)\n", keyName)
				return
			}

			// Get key content
			var keyContent string
			firstArg := strings.TrimSpace(args[0])
			if firstArg == "-" {
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Buffer(make([]byte, 64*1024), 64*1024)
				if !scanner.Scan() {
					fmt.Println("Error: No key provided from stdin")
					return
				}
				keyContent = scanner.Text()
			} else if _, err := os.Stat(firstArg); err == nil {
				data, err := os.ReadFile(firstArg)
				if err != nil {
					fmt.Printf("Error reading key file: %v\n", err)
					return
				}
				keyContent = string(data)
			} else {
				keyContent = firstArg
			}

			// Only the first line counts: a key never spans lines, and extra lines
			// would let a single "key" add arbitrary authorized_keys entries.
			keyContent = strings.TrimSpace(strings.SplitN(strings.TrimSpace(keyContent), "\n", 2)[0])
			key, err := parseAuthorizedKey(keyContent)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Never trust options coming from the input
			key.Options = ""
			if !admin {
				key.Options = restrictedKeyOptions()
			}
			key.Comment = keyName

			path := authorizedKeysPath(cfg.PorticoHome)
			lines, err := readAuthorizedKeys(path)
			if err != nil {
				fmt.Printf("Error reading authorized_keys: %v\n", err)
				return
			}

			for _, line := range lines {
				existing, err := parseAuthorizedKey(line)
				if err != nil {
					continue
				}
				if existing.Data == key.Data {
					fmt.Printf("⚠️  This SSH key already exists (name: %s)\n", existing.Comment)
					return
				}
				if existing.Comment == keyName {
					fmt.Printf("Error: a key named %s already exists\n", keyName)
					return
				}
			}

			if err := writeAuthorizedKeys(path, append(lines, key.String())); err != nil {
				fmt.Printf("Error writing authorized_keys: %v\n", err)
				return
			}

			access := "git push only"
			if admin {
				access = "full shell access"
			}
			fmt.Printf("✅ SSH key added successfully (name: %s, %s, %s)\n", keyName, key.Fingerprint(), access)
		},
	}

	cmd.Flags().BoolVar(&admin, "admin", false, "Grant full shell access instead of git push only")
	return cmd
}
