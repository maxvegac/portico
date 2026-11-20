package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSSHRemoveCmd removes an SSH public key
func NewSSHRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [index|name|fingerprint|key]",
		Short: "Remove an SSH public key",
		Long: `Remove an SSH public key by index or name (from 'portico ssh list'),
by fingerprint (SHA256:...) or by the full public key.

Examples:
  portico ssh remove 1
  portico ssh remove ci-deployment
  portico ssh remove SHA256:abc123...`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			path := authorizedKeysPath(cfg.PorticoHome)
			lines, err := readAuthorizedKeys(path)
			if err != nil {
				fmt.Printf("Error reading authorized_keys: %v\n", err)
				return
			}

			arg := strings.TrimSpace(args[0])
			index, indexErr := strconv.Atoi(arg)
			var argKeyData string
			if k, err := parseAuthorizedKey(arg); err == nil {
				argKeyData = k.Data
			}

			// Exact matches only: a substring match could remove every key
			var kept []string
			var removed []string
			keyIndex := 0
			for _, line := range lines {
				key, err := parseAuthorizedKey(line)
				if err != nil {
					kept = append(kept, line) // keep comments and unknown lines
					continue
				}
				keyIndex++

				match := (indexErr == nil && keyIndex == index) ||
					key.Comment == arg ||
					key.Fingerprint() == arg ||
					key.Data == arg ||
					(argKeyData != "" && key.Data == argKeyData)
				if match {
					removed = append(removed, key.Comment)
					continue
				}
				kept = append(kept, line)
			}

			if len(removed) == 0 {
				fmt.Printf("No key found matching %q. Use 'portico ssh list' to see available keys\n", arg)
				return
			}

			if err := writeAuthorizedKeys(path, kept); err != nil {
				fmt.Printf("Error writing authorized_keys: %v\n", err)
				return
			}

			fmt.Printf("✅ Removed %d SSH key(s): %s\n", len(removed), strings.Join(removed, ", "))
		},
	}
}
