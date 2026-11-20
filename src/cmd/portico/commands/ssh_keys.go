package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/maxvegac/portico/src/internal/util"
)

// authorizedKey is one entry of ~/.ssh/authorized_keys
type authorizedKey struct {
	Options string // e.g. command="...",no-pty (empty for unrestricted keys)
	Type    string // e.g. ssh-ed25519
	Data    string // base64 key blob
	Comment string // key name
}

// knownKeyTypes are the public key algorithms accepted by OpenSSH
var knownKeyTypes = map[string]bool{
	"ssh-rsa":                            true,
	"ssh-dss":                            true,
	"ssh-ed25519":                        true,
	"ecdsa-sha2-nistp256":                true,
	"ecdsa-sha2-nistp384":                true,
	"ecdsa-sha2-nistp521":                true,
	"sk-ssh-ed25519@openssh.com":         true,
	"sk-ecdsa-sha2-nistp256@openssh.com": true,
}

// keyNameRe restricts key names: they are written as the key comment
var keyNameRe = regexp.MustCompile(`^[A-Za-z0-9_.@:+-]{1,64}$`)

// splitOptions splits a leading options field from the rest of an authorized_keys
// line, honouring double quotes (options may contain spaces inside quotes).
func splitOptions(line string) (string, string) {
	inQuotes := false
	for i, r := range line {
		switch {
		case r == '"' && (i == 0 || line[i-1] != '\\'):
			inQuotes = !inQuotes
		case (r == ' ' || r == '\t') && !inQuotes:
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}

// parseAuthorizedKey parses an authorized_keys line (with or without options)
func parseAuthorizedKey(line string) (*authorizedKey, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, fmt.Errorf("empty line")
	}

	key := &authorizedKey{}
	first := strings.Fields(line)[0]
	if !knownKeyTypes[first] {
		key.Options, line = splitOptions(line)
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid SSH key format, expected 'algorithm key-data [comment]'")
	}
	key.Type, key.Data = fields[0], fields[1]
	if len(fields) > 2 {
		key.Comment = strings.Join(fields[2:], " ")
	}

	if !knownKeyTypes[key.Type] {
		return nil, fmt.Errorf("unsupported key type %q", key.Type)
	}
	blob, err := base64.StdEncoding.DecodeString(key.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid key data: %w", err)
	}
	// The blob starts with the length-prefixed algorithm name
	if len(blob) < 4 {
		return nil, fmt.Errorf("invalid key data: too short")
	}
	n := binary.BigEndian.Uint32(blob[:4])
	if uint32(len(blob)-4) < n || !bytes.Equal(blob[4:4+n], []byte(key.Type)) {
		return nil, fmt.Errorf("invalid key data: does not match key type %s", key.Type)
	}
	return key, nil
}

// Fingerprint returns the OpenSSH SHA256 fingerprint of the key
func (k *authorizedKey) Fingerprint() string {
	blob, err := base64.StdEncoding.DecodeString(k.Data)
	if err != nil {
		return "(invalid)"
	}
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Restricted reports whether the key is limited to git operations
func (k *authorizedKey) Restricted() bool {
	return strings.Contains(k.Options, "ssh-command")
}

// String renders the key as an authorized_keys line
func (k *authorizedKey) String() string {
	parts := []string{}
	if k.Options != "" {
		parts = append(parts, k.Options)
	}
	parts = append(parts, k.Type, k.Data)
	if k.Comment != "" {
		parts = append(parts, k.Comment)
	}
	return strings.Join(parts, " ")
}

// restrictedKeyOptions returns the options that limit a key to git push/fetch
func restrictedKeyOptions() string {
	porticoBin := "/usr/local/bin/portico"
	if exe, err := os.Executable(); err == nil {
		porticoBin = exe
	}
	return fmt.Sprintf(`command="%s ssh-command",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty`, porticoBin)
}

// authorizedKeysPath returns the path of the portico user's authorized_keys
func authorizedKeysPath(porticoHome string) string {
	return filepath.Join(porticoHome, ".ssh", "authorized_keys")
}

// readAuthorizedKeys returns the raw lines of authorized_keys (nil if missing)
func readAuthorizedKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

// writeAuthorizedKeys atomically rewrites authorized_keys (sshd ignores it if permissions are too open)
func writeAuthorizedKeys(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := util.WriteFileAtomic(path, []byte(content), 0o600); err != nil {
		return err
	}
	_ = util.FixFileOwnership(path)
	return nil
}
