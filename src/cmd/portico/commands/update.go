package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Constants for repeated strings
const (
	archAmd64 = "amd64"
	archArm64 = "arm64"
	arch386   = "386"
)

// Release represents a GitHub release
type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Prerelease  bool    `json:"prerelease"`
	Assets      []Asset `json:"assets"`
	PublishedAt string  `json:"published_at"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}

// UpdateManager handles auto-update functionality
type UpdateManager struct {
	RepoOwner string
	RepoName  string
	IsDev     bool
}

// NewUpdateManager creates a new UpdateManager
func NewUpdateManager(repoOwner, repoName string, isDev bool) *UpdateManager {
	return &UpdateManager{
		RepoOwner: repoOwner,
		RepoName:  repoName,
		IsDev:     isDev,
	}
}

// CheckForUpdates checks if there's a newer version available
func (um *UpdateManager) CheckForUpdates() (*Release, error) {
	if um.IsDev {
		return um.checkForDevUpdates()
	}
	return um.checkForStableUpdates()
}

// checkForDevUpdates checks for development updates
func (um *UpdateManager) checkForDevUpdates() (*Release, error) {
	// First try to get dev-latest
	devRelease := um.getDevLatestRelease()
	return devRelease, nil
}

// checkForStableUpdates checks for stable updates
func (um *UpdateManager) checkForStableUpdates() (*Release, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", um.RepoOwner, um.RepoName)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("error decoding releases: %w", err)
	}

	// Filter for stable releases only
	var stableReleases []Release
	for _, release := range releases {
		if !release.Prerelease {
			stableReleases = append(stableReleases, release)
		}
	}

	if len(stableReleases) == 0 {
		// If no stable releases, try dev-latest as fallback
		devRelease := um.getDevLatestRelease()
		return devRelease, nil
	}

	// Return the latest stable release
	return &stableReleases[0], nil
}

// getDevLatestRelease gets the dev-latest release
func (um *UpdateManager) getDevLatestRelease() *Release {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/dev-latest", um.RepoOwner, um.RepoName)
	client := &http.Client{Timeout: 10 * time.Second}
	if req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, http.NoBody); err == nil {
		if resp, err := client.Do(req); err == nil {
			defer func() { _ = resp.Body.Close() }()
			var release Release
			if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&release) == nil && len(release.Assets) > 0 {
				return &release
			}
		}
	}

	// Fallback: build the download URLs directly
	downloadURL := func(name string) string {
		return fmt.Sprintf("https://github.com/%s/%s/releases/download/dev-latest/%s", um.RepoOwner, um.RepoName, name)
	}
	return &Release{
		TagName:    "dev-latest",
		Name:       "Development Latest",
		Prerelease: true,
		Assets: []Asset{
			{Name: um.getAssetName(), BrowserDownloadURL: downloadURL(um.getAssetName())},
			{Name: checksumsAsset, BrowserDownloadURL: downloadURL(checksumsAsset)},
		},
		PublishedAt: time.Now().Format(time.RFC3339),
	}
}

// GetCurrentVersion gets the current version of Portico
func (um *UpdateManager) GetCurrentVersion() (string, error) {
	if Version == "" {
		return "dev", nil
	}
	return Version, nil
}

// checksumsAsset is the release asset with "sha256  filename" lines (published by CI)
const checksumsAsset = "checksums.txt"

// expectedChecksum returns the published SHA256 of the binary for this platform
func (um *UpdateManager) expectedChecksum(release *Release) (string, error) {
	var checksums *Asset
	for i := range release.Assets {
		if release.Assets[i].Name == checksumsAsset {
			checksums = &release.Assets[i]
		}
	}
	if checksums == nil {
		return "", fmt.Errorf("release %s has no %s", release.TagName, checksumsAsset)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", checksums.BrowserDownloadURL, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error downloading %s: %w", checksumsAsset, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s failed with status %d", checksumsAsset, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	return parseChecksum(string(data), um.getAssetName())
}

// parseChecksum finds the checksum of name in sha256sum output
func parseChecksum(checksums, name string) (string, error) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", name)
}

// fileSHA256 returns the hex SHA256 of a file
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsUpToDate reports whether the running binary already matches the release.
// The checksum comparison also works for dev-latest, whose tag never changes.
func (um *UpdateManager) IsUpToDate(release *Release) bool {
	if expected, err := um.expectedChecksum(release); err == nil {
		if exe, err := os.Executable(); err == nil {
			if current, err := fileSHA256(exe); err == nil {
				return current == expected
			}
		}
	}
	current, _ := um.GetCurrentVersion()
	return release.TagName == current && release.TagName != "dev-latest"
}

// DownloadRelease downloads the latest release binary
func (um *UpdateManager) DownloadRelease(release *Release) error {
	targetAsset, err := um.findTargetAsset(release)
	if err != nil {
		return err
	}

	expected, err := um.expectedChecksum(release)
	if err != nil {
		return fmt.Errorf("cannot verify download, aborting: %w", err)
	}

	fmt.Printf("Downloading %s...\n", targetAsset.Name)

	tmpFile, err := um.downloadBinary(targetAsset)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	actual, err := fileSHA256(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("error hashing download: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s), aborting", targetAsset.Name, expected, actual)
	}
	fmt.Println("Checksum verified.")

	return um.installBinary(tmpFile.Name())
}

// findTargetAsset finds the appropriate asset for the current platform
func (um *UpdateManager) findTargetAsset(release *Release) (*Asset, error) {
	assetName := um.getAssetName()

	for i, asset := range release.Assets {
		if asset.Name == assetName {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("no suitable binary found for %s", runtime.GOOS+"-"+runtime.GOARCH)
}

// downloadBinary downloads the binary to a temporary file
func (um *UpdateManager) downloadBinary(asset *Asset) (*os.File, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(context.Background(), "GET", asset.BrowserDownloadURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading binary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create the temporary file next to the current binary: os.Rename cannot move
	// files across filesystems, and /tmp is often a tmpfs on Linux.
	tmpDir := ""
	if exe, err := os.Executable(); err == nil {
		tmpDir = filepath.Dir(exe)
	}
	tmpFile, err := os.CreateTemp(tmpDir, ".portico-update-*")
	if err != nil && tmpDir != "" {
		tmpFile, err = os.CreateTemp("", "portico-update-*")
	}
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	// Copy downloaded content to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("error writing to temp file: %w", err)
	}
	if cerr := tmpFile.Close(); cerr != nil {
		return nil, fmt.Errorf("error closing temp file: %w", cerr)
	}

	return tmpFile, nil
}

// installBinary installs the downloaded binary
func (um *UpdateManager) installBinary(tmpFilePath string) error {
	// Get current executable path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	// Make temp file executable
	if err := os.Chmod(tmpFilePath, 0o755); err != nil {
		return fmt.Errorf("error making temp file executable: %w", err)
	}

	// Replace current binary using atomic update strategy
	if err := atomicReplaceBinary(tmpFilePath, currentPath); err != nil {
		return fmt.Errorf("error replacing binary: %w", err)
	}

	return nil
}

// atomicReplaceBinary replaces the currently running executable atomically
func atomicReplaceBinary(newBinary, currentBinary string) error {
	// Get directory and filename of current binary
	currentDir := filepath.Dir(currentBinary)
	currentName := filepath.Base(currentBinary)

	// Create a new name for the current binary (add .old suffix)
	oldBinary := filepath.Join(currentDir, currentName+".old")

	// Step 1: Move current binary to .old (this works because we're not deleting it)
	if err := os.Rename(currentBinary, oldBinary); err != nil {
		return fmt.Errorf("error moving current binary to .old: %w", err)
	}

	// Step 2: Move new binary to the original location
	if err := os.Rename(newBinary, currentBinary); err != nil {
		// If this fails, try to restore the original binary
		if restoreErr := os.Rename(oldBinary, currentBinary); restoreErr != nil {
			return fmt.Errorf("error moving new binary to final location: %w (restore also failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("error moving new binary to final location: %w", err)
	}

	// Step 3: Clean up the old binary (optional, can be left for manual cleanup)
	// os.Remove(oldBinary)

	return nil
}

// atomicCopy performs an atomic file copy to avoid "text file busy" errors
func atomicCopy(src, dst string, mode os.FileMode) error { // nolint:unused
	// Create a temporary file in the same directory as destination
	dstDir := filepath.Dir(dst)
	tmpFile := filepath.Join(dstDir, ".portico-update-tmp-"+filepath.Base(dst))

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// Create temporary destination file
	tmpFileHandle, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}
	defer func() { _ = tmpFileHandle.Close() }()

	// Copy file contents to temporary file
	_, err = io.Copy(tmpFileHandle, srcFile)
	if err != nil {
		_ = os.Remove(tmpFile) // Clean up on error
		return fmt.Errorf("error copying file contents: %w", err)
	}

	// Ensure temporary file is written to disk
	if err := tmpFileHandle.Sync(); err != nil {
		_ = os.Remove(tmpFile) // Clean up on error
		return fmt.Errorf("error syncing temporary file: %w", err)
	}
	if err := tmpFileHandle.Close(); err != nil { // Close before rename
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error closing temporary file: %w", err)
	}

	// Atomically replace the destination file
	if err := os.Rename(tmpFile, dst); err != nil {
		_ = os.Remove(tmpFile) // Clean up on error
		return fmt.Errorf("error replacing destination file: %w", err)
	}

	return nil
}

// getAssetName returns the expected asset name for the current platform
func (um *UpdateManager) getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map architecture names to match install.sh
	switch arch {
	case archAmd64:
		arch = archAmd64
	case archArm64:
		arch = archArm64
	case arch386:
		arch = arch386
	}

	if um.IsDev {
		return fmt.Sprintf("portico-dev-latest-%s-%s", os, arch)
	}
	return fmt.Sprintf("portico-%s-%s", os, arch)
}

// NewUpdateCmd creates the update command
func NewUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update Portico to the latest version",
		Long:  `Check for updates and automatically download and install the latest version of Portico.`,
		Run: func(cmd *cobra.Command, _ []string) {
			runUpdateCommand(cmd)
		},
	}
}

// runUpdateCommand handles the update command logic
func runUpdateCommand(cmd *cobra.Command) {
	// Determine if we're in dev mode
	// Dev builds follow dev-latest unless asked otherwise
	isDev := strings.HasPrefix(Version, "dev")
	if devFlag, _ := cmd.Flags().GetBool("dev"); devFlag {
		isDev = true
	}

	// Create update manager
	updateManager := NewUpdateManager("maxvegac", "portico", isDev)

	// Get current version
	currentVersion, err := updateManager.GetCurrentVersion()
	if err != nil {
		fmt.Printf("Error getting current version: %v\n", err)
		return
	}

	fmt.Printf("Current version: %s\n", currentVersion)

	// Check for updates
	fmt.Println("Checking for updates...")
	latestRelease, err := updateManager.CheckForUpdates()
	if err != nil {
		fmt.Printf("Error checking for updates: %v\n", err)
		return
	}

	fmt.Printf("Latest version: %s\n", latestRelease.TagName)

	// Check if update is needed
	if updateManager.IsUpToDate(latestRelease) {
		fmt.Println("✅ You're already running the latest version!")
		return
	}

	// Ask for confirmation
	fmt.Printf("Update available: %s -> %s\n", currentVersion, latestRelease.TagName)

	fmt.Print("Do you want to update? (y/N): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		fmt.Println("Update canceled.")
		return
	}

	if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
		fmt.Println("Update canceled.")
		return
	}

	// Download and install update
	if err := updateManager.DownloadRelease(latestRelease); err != nil {
		fmt.Printf("Error updating: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully updated to %s!\n", latestRelease.TagName)
	fmt.Println("Please restart your terminal or run 'portico version' to verify the update.")
}
