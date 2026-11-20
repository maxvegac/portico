package commands

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

// testKey builds a syntactically valid ssh-ed25519 public key
func testKey(seed byte) string {
	keyType := "ssh-ed25519"
	blob := make([]byte, 4, 4+len(keyType)+4+32)
	binary.BigEndian.PutUint32(blob, uint32(len(keyType)))
	blob = append(blob, keyType...)
	blob = binary.BigEndian.AppendUint32(blob, 32)
	for i := 0; i < 32; i++ {
		blob = append(blob, seed+byte(i))
	}
	return keyType + " " + base64.StdEncoding.EncodeToString(blob)
}

func TestParseAuthorizedKey(t *testing.T) {
	plain := testKey(1) + " my key"
	k, err := parseAuthorizedKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	if k.Type != "ssh-ed25519" || k.Comment != "my key" || k.Options != "" || k.Restricted() {
		t.Errorf("unexpected parse: %+v", k)
	}
	if !strings.HasPrefix(k.Fingerprint(), "SHA256:") {
		t.Errorf("fingerprint = %q", k.Fingerprint())
	}

	withOpts := `command="/usr/local/bin/portico ssh-command",no-pty ` + testKey(2) + " ci"
	k, err = parseAuthorizedKey(withOpts)
	if err != nil {
		t.Fatal(err)
	}
	if !k.Restricted() || k.Comment != "ci" || k.String() != withOpts {
		t.Errorf("unexpected parse: %+v", k)
	}

	for _, bad := range []string{"", "# comment", "ssh-rsa", "foo AAAA", "ssh-ed25519 not-base64!!", "ssh-rsa " + strings.Fields(testKey(3))[1]} {
		if _, err := parseAuthorizedKey(bad); err == nil {
			t.Errorf("parseAuthorizedKey(%q) succeeded, want error", bad)
		}
	}
}

func TestParseGitSSHCommand(t *testing.T) {
	tests := []struct {
		in, service, app string
	}{
		{"git-receive-pack 'my-app.git'", "git-receive-pack", "my-app"},
		{"git-receive-pack '/my-app.git'", "git-receive-pack", "my-app"},
		{"git-receive-pack '~/repos/my-app.git'", "git-receive-pack", "my-app"},
		{"git upload-pack 'my-app'", "git-upload-pack", "my-app"},
	}
	for _, tt := range tests {
		service, app, err := parseGitSSHCommand(tt.in)
		if err != nil || service != tt.service || app != tt.app {
			t.Errorf("parseGitSSHCommand(%q) = %q, %q, %v", tt.in, service, app, err)
		}
	}

	for _, bad := range []string{"bash", "ls -la", "git-receive-pack '../etc.git'", "git-receive-pack 'a/b.git'", "rm -rf /", "git-receive-pack 'x.git'; id"} {
		if _, _, err := parseGitSSHCommand(bad); err == nil {
			t.Errorf("parseGitSSHCommand(%q) succeeded, want error", bad)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	sums := "abc123  portico-linux-amd64\nDEF456 *portico-dev-latest-linux-amd64\n"
	if got, err := parseChecksum(sums, "portico-linux-amd64"); err != nil || got != "abc123" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := parseChecksum(sums, "portico-dev-latest-linux-amd64"); err != nil || got != "def456" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := parseChecksum(sums, "portico-linux"); err == nil {
		t.Error("partial name matched")
	}
}
