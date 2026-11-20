package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewSSHCommandCmd is the forced command of restricted SSH keys: it only allows
// git push/fetch to Portico repositories.
func NewSSHCommandCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "ssh-command",
		Short:  "Handle commands of restricted SSH keys (internal use)",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			if err := runSSHCommand(os.Getenv("SSH_ORIGINAL_COMMAND")); err != nil {
				fmt.Fprintf(os.Stderr, "portico: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

// parseGitSSHCommand parses `git-receive-pack '<path>'` (or upload-pack) and
// returns the git service and app name.
func parseGitSSHCommand(original string) (string, string, error) {
	fields := strings.Fields(original)
	if len(fields) == 3 && fields[0] == "git" {
		fields = []string{"git-" + fields[1], fields[2]}
	}
	if len(fields) != 2 {
		return "", "", fmt.Errorf("only git push/fetch is allowed with this key (git push portico@<server>:<app>.git)")
	}

	service := fields[0]
	if service != "git-receive-pack" && service != "git-upload-pack" {
		return "", "", fmt.Errorf("only git push/fetch is allowed with this key")
	}

	repo := strings.Trim(fields[1], `'"`)
	repo = strings.TrimPrefix(repo, "~/")
	repo = strings.TrimPrefix(repo, "/")
	repo = strings.TrimPrefix(repo, "repos/")
	repo = strings.TrimSuffix(strings.TrimSuffix(repo, "/"), ".git")

	if err := util.ValidateAppName(repo); err != nil {
		return "", "", fmt.Errorf("invalid repository %q: %w", fields[1], err)
	}
	return service, repo, nil
}

func runSSHCommand(original string) error {
	if original == "" {
		return fmt.Errorf("interactive login is not allowed with this key; use: git push portico@<server>:<app>.git")
	}

	service, appName, err := parseGitSSHCommand(original)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	repoDir := filepath.Join(cfg.PorticoHome, "repos", appName+".git")
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		if service != "git-receive-pack" {
			return fmt.Errorf("repository %s.git does not exist", appName)
		}
		// First push to a new app: create its repository (the hook creates the app)
		if err := createGitRepo(cfg.PorticoHome, appName); err != nil {
			return fmt.Errorf("error creating repository: %w", err)
		}
	}

	cmd := exec.Command(service, repoDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
