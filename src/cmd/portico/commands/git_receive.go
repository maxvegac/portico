package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// zeroSHA is the "new revision" git reports when a ref is deleted
const zeroSHA = "0000000000000000000000000000000000000000"

// deployRefs are the branches that trigger a deployment when pushed
var deployRefs = map[string]bool{
	"refs/heads/main":   true,
	"refs/heads/master": true,
}

// NewGitReceiveCmd handles git post-receive hook
func NewGitReceiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "git-receive",
		Short:  "Handle git post-receive hook (internal use)",
		Hidden: true, // Hide from help since it's only used by git hooks
		Args:   cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			if err := runGitReceive(); err != nil {
				fmt.Printf("❌ %v\n", err)
				os.Exit(1)
			}
		},
	}

	return cmd
}

func runGitReceive() error {
	// Get app name from current directory (git repo)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %w", err)
	}

	// Extract app name from repo directory (e.g., /home/portico/repos/my-app.git -> my-app)
	appName := strings.TrimSuffix(filepath.Base(cwd), ".git")
	if err := util.ValidateAppName(appName); err != nil {
		return fmt.Errorf("could not determine app name from repository directory: %w", err)
	}

	// Read git push information from stdin: "<old-sha> <new-sha> <refname>" per ref.
	// Only the deploy branch (main or master) is deployed; other refs are just stored.
	var newRev, refname string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 || parts[1] == zeroSHA {
			continue
		}
		if deployRefs[parts[2]] {
			newRev, refname = parts[1], parts[2]
		} else {
			fmt.Printf("-----> Skipping %s (only main/master are deployed)\n", parts[2])
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}
	if newRev == "" {
		fmt.Println("Nothing to deploy (no updated refs).")
		return nil
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Create temporary directory in /home/portico/.tmp
	tmpDir := filepath.Join(cfg.PorticoHome, ".tmp", fmt.Sprintf("%s-%d", appName, os.Getpid()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Warning: could not remove temporary directory: %v\n", err)
		}
	}()

	// Export the pushed revision (by SHA, so it works for branches and tags alike)
	fmt.Printf("-----> Received %s (%s)\n", refname, shortSHA(newRev))
	checkout := exec.Command("git", "--work-tree", tmpDir, "--git-dir", cwd, "checkout", "-f", newRev, "--", ".")
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("error checking out code: %w", err)
	}

	// Check for Dockerfile
	if _, err := os.Stat(filepath.Join(tmpDir, "Dockerfile")); os.IsNotExist(err) {
		return fmt.Errorf("no Dockerfile found in the repository root")
	}

	// Build Docker image
	imageName := fmt.Sprintf("portico-%s:latest", appName)
	fmt.Printf("-----> Building Docker image: %s\n", imageName)
	if err := buildImage(imageName, filepath.Join(tmpDir, "Dockerfile"), tmpDir, nil); err != nil {
		return fmt.Errorf("error building Docker image: %w", err)
	}

	// Deploy
	am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
	a, err := loadOrCreateApp(cfg, am, appName)
	if err != nil {
		return err
	}
	serviceName, _ := setServiceImage(a, "", imageName)

	fmt.Printf("-----> Deploying service %s\n", serviceName)
	if err := applyApp(cfg, am, a); err != nil {
		return err
	}

	fmt.Printf("✅ Application %s deployed successfully!\n", appName)
	if url := appURL(a); url != "" {
		fmt.Printf("       %s\n", url)
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
