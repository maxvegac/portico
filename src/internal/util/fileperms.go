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

	// Change ownership recursively. Lchown: never follow symlinks out of the tree.
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, uid, gid)
	})
}

// WriteFileAtomic writes data to a temporary file in the same directory and renames it
// over path, so readers never observe a partially written file.
// Note: when path is bind-mounted as a single file into a container, prefer os.WriteFile
// (rename replaces the inode and the container would keep seeing the old file).
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
