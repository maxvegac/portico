package commands

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

// domainRe matches a DNS hostname (optionally with a leading "*." wildcard label)
var domainRe = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// validateDomain checks that a domain is a plain hostname. The value is written
// into the Caddyfile, so anything else could inject Caddy configuration.
func validateDomain(domain string) error {
	if len(domain) > 253 || !domainRe.MatchString(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}
	return nil
}

// NewDomainsCmd is the root command for `domains [app-name] add|remove <domain>`
func NewDomainsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "domains [app-name]",
		Short: "Manage application domains",
		Long:  "Manage domains for an application.\n\nExamples:\n  portico domains my-app add example.com\n  portico domains my-app remove example.com",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("domains"),
	}
}

// getAppNameFromDomainsArgs extracts app-name from domains command arguments
func getAppNameFromDomainsArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}
