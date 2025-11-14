package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// FixFileOwnership changes file ownership to portico user if running as root
func FixFileOwnership(filePath string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return nil // Not root, no need to fix ownership
	}

	// Get portico user info
	porticoUser, err := user.Lookup("portico")
	if err != nil {
		// Portico user doesn't exist, skip ownership fix
		return nil
	}

	// Parse UID and GID
	var uid, gid int
	if _, err := fmt.Sscanf(porticoUser.Uid, "%d", &uid); err != nil {
		return fmt.Errorf("error parsing portico UID: %w", err)
	}
	if _, err := fmt.Sscanf(porticoUser.Gid, "%d", &gid); err != nil {
		return fmt.Errorf("error parsing portico GID: %w", err)
	}

	// Change ownership
	return os.Chown(filePath, uid, gid)
}

// FixDirOwnership recursively changes directory ownership to portico user if running as root
func FixDirOwnership(dirPath string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return nil // Not root, no need to fix ownership
	}

	// Get portico user info
	porticoUser, err := user.Lookup("portico")
	if err != nil {
		// Portico user doesn't exist, skip ownership fix
		return nil
	}

	// Parse UID and GID
	var uid, gid int
	if _, err := fmt.Sscanf(porticoUser.Uid, "%d", &uid); err != nil {
		return fmt.Errorf("error parsing portico UID: %w", err)
	}
	if _, err := fmt.Sscanf(porticoUser.Gid, "%d", &gid); err != nil {
		return fmt.Errorf("error parsing portico GID: %w", err)
	}

	// Change ownership recursively
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}
