package commands

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxvegac/portico/src/internal/addon"
)

func TestToolEnvironmentMongoExpress(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Credentials with URL-reserved characters must be escaped
	_ = os.WriteFile(filepath.Join(dir, "secrets", "db_user"), []byte("ad@min\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "secrets", "db_password"), []byte("p:a/ss"), 0o600)

	target := &toolTargetInfo{Instance: addon.Instance{Name: "mymongo"}, Host: "addon-mymongo", Port: 27017, Dir: dir}
	env, err := toolEnvironment("mongo-express", "secret", target)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(env["ME_CONFIG_MONGODB_URL"])
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	if u.Host != "addon-mymongo:27017" || u.User.Username() != "ad@min" || pw != "p:a/ss" || u.Query().Get("authSource") != "admin" {
		t.Errorf("unexpected mongo URL %q", env["ME_CONFIG_MONGODB_URL"])
	}
	if env["ME_CONFIG_BASICAUTH_PASSWORD"] != "secret" {
		t.Error("mongo-express must use the admin password")
	}

	if _, err := toolEnvironment("mongo-express", "secret", &toolTargetInfo{Dir: t.TempDir()}); err == nil {
		t.Error("expected an error when the target credentials are missing")
	}
}

func TestRenderToolCaddyfile(t *testing.T) {
	inst := addon.Instance{Name: "dbadmin", Type: "adminer", Domain: "db.example.com"}
	out := renderToolCaddyfile(inst, 8080, "$2a$10$hash")
	for _, want := range []string{"db.example.com {", "basic_auth {", "admin $2a$10$hash", "reverse_proxy addon-dbadmin:8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("Caddyfile missing %q:\n%s", want, out)
		}
	}
}

func TestWriteEnvFileRejectsNewlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.env")
	if err := writeEnvFile(path, map[string]string{"A": "x\nB=y"}); err == nil {
		t.Error("expected an error for a value with a newline")
	}
	if err := writeEnvFile(path, map[string]string{"B": "2", "A": "1"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "A=1\nB=2\n" {
		t.Errorf("unexpected env file %q", data)
	}
}

func TestCreateDatabaseQueryGrantsMySQLUser(t *testing.T) {
	q := createDatabaseQuery("mariadb", "shop", "admin")
	if !strings.Contains(q, "CREATE DATABASE IF NOT EXISTS `shop`") || !strings.Contains(q, "GRANT ALL PRIVILEGES ON `shop`.* TO 'admin'@'%'") {
		t.Errorf("unexpected query %q", q)
	}
	// An unsafe user name is never interpolated
	if q := createDatabaseQuery("mysql", "shop", "x'; DROP"); strings.Contains(q, "GRANT") {
		t.Errorf("unsafe user interpolated: %q", q)
	}
	if q := createDatabaseQuery("postgresql", "shop", "admin"); q != `CREATE DATABASE "shop"` {
		t.Errorf("unexpected query %q", q)
	}
}
