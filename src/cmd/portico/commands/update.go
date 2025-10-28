package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Constants for repeated strings
const (
	defaultVersion = "1.0.0"
	archAmd64      = "amd64"
	archArm64      = "arm64"
	arch386        = "386"
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
	defer resp.Body.Close()

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
	// Create a mock release for dev-latest
	return &Release{
		TagName:    "dev-latest",
		Name:       "Development Latest",
		Prerelease: true,
		Assets: []Asset{
			{
				Name: um.getAssetName(),
				BrowserDownloadURL: fmt.Sprintf(
					"https://github.com/%s/%s/releases/download/dev-latest/%s",
					um.RepoOwner, um.RepoName, um.getAssetName(),
				),
				Size: 0, // Unknown size
			},
		},
		PublishedAt: time.Now().Format(time.RFC3339),
	}
}

// GetCurrentVersion gets the current version of Portico
func (um *UpdateManager) GetCurrentVersion() (string, error) {
	// Try to get git tag first
	if tag, err := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(tag)), nil
	}

	// If no tag, use commit hash
	if hash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(hash)), nil
	}

	// Fallback to hardcoded version
	return defaultVersion, nil
}

// DownloadRelease downloads the latest release binary
func (um *UpdateManager) DownloadRelease(release *Release) error {
	// Find the appropriate asset for the current platform
	assetName := um.getAssetName()
	var targetAsset *Asset

	for i, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			targetAsset = &release.Assets[i]
			break
		}
	}

	if targetAsset == nil {
		return fmt.Errorf("no suitable binary found for %s", runtime.GOOS+"-"+runtime.GOARCH)
	}

	fmt.Printf("Downloading %s...\n", targetAsset.Name)

	// Download the binary
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", targetAsset.BrowserDownloadURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "portico-update-*")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Copy downloaded content to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("error writing to temp file: %w", err)
	}
	tmpFile.Close()

	// Get current executable path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	// Make temp file executable
	if err := os.Chmod(tmpFile.Name(), 0o755); err != nil {
		return fmt.Errorf("error making temp file executable: %w", err)
	}

	// Replace current binary
	if err := copyFile(tmpFile.Name(), currentPath); err != nil {
		return fmt.Errorf("error replacing binary: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst, handling cross-device issues and permissions
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("error getting source file info: %w", err)
	}

	// Check if destination requires sudo
	needsSudo := needsElevatedPermissions(dst)

	if needsSudo {
		return copyFileWithSudo(src, dst, srcInfo.Mode())
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file contents: %w", err)
	}

	// Ensure destination file is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("error syncing destination file: %w", err)
	}

	return nil
}

// needsElevatedPermissions checks if the destination path requires elevated permissions
func needsElevatedPermissions(path string) bool {
	// Check if the directory is writable by current user
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err == nil {
		// Check if directory is writable
		if info.Mode()&0o200 == 0 {
			return true
		}
	}

	// Check common system directories that require sudo
	systemDirs := []string{
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/sbin",
		"/usr/sbin",
	}

	for _, sysDir := range systemDirs {
		if strings.HasPrefix(path, sysDir) {
			return true
		}
	}

	return false
}

// copyFileWithSudo copies a file using sudo when elevated permissions are needed
func copyFileWithSudo(src, dst string, mode os.FileMode) error {
	// First, copy to a temporary location that doesn't require sudo
	tmpDir := os.TempDir()
	tmpDst := filepath.Join(tmpDir, "portico-update-tmp")

	// Copy to temp location first
	if err := copyFileDirect(src, tmpDst, mode); err != nil {
		return fmt.Errorf("error copying to temp location: %w", err)
	}
	defer os.Remove(tmpDst) // Clean up temp file

	// Use sudo to copy from temp to final destination
	if err := copyFileWithSudoCmd(tmpDst, dst); err != nil {
		return fmt.Errorf("error copying with sudo: %w", err)
	}

	// Set permissions with sudo
	if err := setFilePermissions(dst, mode); err != nil {
		return fmt.Errorf("error setting permissions with sudo: %w", err)
	}

	return nil
}

// setFilePermissions safely sets file permissions using sudo
func setFilePermissions(filePath string, mode os.FileMode) error {
	// Validate file path to prevent path traversal
	if !filepath.IsAbs(filePath) {
		filePath, _ = filepath.Abs(filePath)
	}

	// Validate that the path is safe (no dangerous characters)
	if strings.Contains(filePath, "..") || strings.Contains(filePath, "~") {
		return fmt.Errorf("unsafe file path: %s", filePath)
	}

	// Convert mode to octal string safely
	permStr := fmt.Sprintf("%o", mode)

	// Validate that the permission string is safe (only digits 0-7)
	for _, char := range permStr {
		if char < '0' || char > '7' {
			return fmt.Errorf("invalid permission mode: %s", permStr)
		}
	}

	// Use exec.Command with validated arguments
	cmd := exec.Command("sudo", "chmod", permStr, filePath)
	return cmd.Run()
}

// copyFileWithSudoCmd safely copies a file using sudo
func copyFileWithSudoCmd(src, dst string) error {
	// Validate source path
	if !filepath.IsAbs(src) {
		src, _ = filepath.Abs(src)
	}

	// Validate destination path
	if !filepath.IsAbs(dst) {
		dst, _ = filepath.Abs(dst)
	}

	// Validate that paths are safe (no dangerous characters)
	if strings.Contains(src, "..") || strings.Contains(src, "~") ||
		strings.Contains(dst, "..") || strings.Contains(dst, "~") {
		return fmt.Errorf("unsafe file path: src=%s, dst=%s", src, dst)
	}

	// Use exec.Command with validated arguments
	cmd := exec.Command("sudo", "cp", src, dst)
	return cmd.Run()
}

// copyFileDirect copies a file directly without permission checks
func copyFileDirect(src, dst string, mode os.FileMode) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file contents: %w", err)
	}

	// Ensure destination file is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("error syncing destination file: %w", err)
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
	isDev := false
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
	if latestRelease.TagName == currentVersion {
		fmt.Println("✅ You're already running the latest version!")
		return
	}

	// Ask for confirmation
	fmt.Printf("Update available: %s -> %s\n", currentVersion, latestRelease.TagName)

	// Check if sudo will be needed
	currentPath, err := os.Executable()
	if err == nil && needsElevatedPermissions(currentPath) {
		fmt.Println("⚠️  This update will require sudo privileges")
	}

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
