package commands

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewSetExternalIPCmd sets the external IP address for sslip.io domain generation
func NewSetExternalIPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "external-ip [ip-address]",
		Short: "Set external IP address for sslip.io domain generation",
		Long: `Set the external IP address used for generating sslip.io domains.
		
This IP will be used when generating default domains in the format: appname.IP.sslip.io
If not set, Portico will attempt to detect the IP automatically (local first, then external via ipinfo.io).

Examples:
  portico set external-ip 145.79.197.47
  portico set external-ip auto  (to enable auto-detection)`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ipArg := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Handle "auto" to enable auto-detection
			if ipArg == "auto" {
				cfg.ExternalIP = ""
				if err := cfg.SaveConfig(); err != nil {
					fmt.Printf("Error saving config: %v\n", err)
					return
				}
				fmt.Println("External IP set to auto-detection mode")
				return
			}

			// Validate IP format
			if err := validateIP(ipArg); err != nil {
				fmt.Printf("Error: invalid IP address format: %v\n", err)
				fmt.Println("Please provide a valid IPv4 address (e.g., 145.79.197.47)")
				return
			}

			cfg.ExternalIP = ipArg
			if err := cfg.SaveConfig(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				return
			}

			fmt.Printf("External IP set to %s\n", ipArg)
			fmt.Println("This IP will be used for generating sslip.io domains for new apps")
		},
	}
}

// validateIP validates that the string is a valid IPv4 address
func validateIP(ip string) error {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP format")
	}
	if parsedIP.To4() == nil {
		return fmt.Errorf("only IPv4 addresses are supported")
	}
	return nil
}
