package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxvegac/portico/src/internal/docker"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	// Templates dir doesn't exist: embedded templates are used
	return NewManager(filepath.Join(dir, "apps"), filepath.Join(dir, "templates"))
}

func createTestApp(t *testing.T, am *Manager, a *App) {
	t.Helper()
	if err := am.CreateAppDirectories(a.Name); err != nil {
		t.Fatal(err)
	}
	dm := docker.NewManager("")
	if err := dm.GenerateDockerCompose(filepath.Join(am.AppsDir, a.Name), nil, a.Metadata()); err != nil {
		t.Fatal(err)
	}
	if err := am.SaveApp(a); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidAppNamesAreRejected(t *testing.T) {
	am := newTestManager(t)
	for _, name := range []string{"", "..", "../outside", "a/b"} {
		if err := am.DeleteApp(name); err == nil {
			t.Errorf("DeleteApp(%q) succeeded, want error", name)
		}
		if _, err := am.LoadApp(name); err == nil {
			t.Errorf("LoadApp(%q) succeeded, want error", name)
		}
		if err := am.CreateAppDirectories(name); err == nil {
			t.Errorf("CreateAppDirectories(%q) succeeded, want error", name)
		}
	}
	// The apps dir itself must survive DeleteApp("")
	if err := os.MkdirAll(am.AppsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = am.DeleteApp("")
	if _, err := os.Stat(am.AppsDir); err != nil {
		t.Fatalf("apps dir was removed: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	am := newTestManager(t)
	a := &App{
		Name:        "demo",
		Domain:      "demo.example.com",
		Port:        3000,
		HttpService: "api",
		Services: []Service{
			{
				Name:  "worker",
				Image: "busybox",
				Environment: map[string]string{
					"URL":     "postgres://u:p@h:5432/db?sslmode=disable",
					"COMMENT": "value # not a comment",
					"QUOTE":   `say "hi": now`,
					"EMPTY":   "",
				},
				Volumes: []string{"/data:/data", "/data:/data"},
			},
			{Name: "api", Image: "nginx:1.27", Replicas: 3},
		},
	}
	createTestApp(t, am, a)

	loaded, err := am.LoadApp("demo")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Domain != a.Domain || loaded.Port != 3000 || loaded.HttpService != "api" {
		t.Errorf("metadata not preserved: %+v", loaded)
	}
	if loaded.Services[0].Name != "api" || loaded.Services[1].Name != "worker" {
		t.Errorf("services not sorted: %v, %v", loaded.Services[0].Name, loaded.Services[1].Name)
	}
	if loaded.Services[0].Replicas != 3 {
		t.Errorf("replicas not preserved: %d", loaded.Services[0].Replicas)
	}
	worker := loaded.FindService("worker")
	for k, v := range a.Services[0].Environment {
		if worker.Environment[k] != v {
			t.Errorf("env %s = %q, want %q", k, worker.Environment[k], v)
		}
	}
	if len(worker.Volumes) != 1 || worker.Volumes[0] != "/data:/data" {
		t.Errorf("volumes = %v, want only the user volume once", worker.Volumes)
	}

	// Saving again must not duplicate managed mounts (logs/secrets)
	if err := am.SaveApp(loaded); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(am.AppsDir, "demo", "docker-compose.yml"))
	if n := strings.Count(string(data), ":/app/logs"); n != 2 { // one per service
		t.Errorf("logs mount appears %d times, want 2:\n%s", n, data)
	}
	if !strings.Contains(string(data), "demo-api") {
		t.Errorf("network alias demo-api missing:\n%s", data)
	}
}

func TestDeletionsArePersisted(t *testing.T) {
	am := newTestManager(t)
	createTestApp(t, am, &App{
		Name:   "demo",
		Domain: "demo.example.com",
		Port:   8080,
		Services: []Service{{
			Name:        "web",
			Image:       "nginx",
			Environment: map[string]string{"KEEP": "1", "REMOVE": "2"},
			Volumes:     []string{"/keep:/keep", "/remove:/remove"},
		}},
	})

	a, err := am.LoadApp("demo")
	if err != nil {
		t.Fatal(err)
	}
	delete(a.Services[0].Environment, "REMOVE")
	a.Services[0].Volumes = []string{"/keep:/keep"}
	if err := am.SaveApp(a); err != nil {
		t.Fatal(err)
	}

	a, _ = am.LoadApp("demo")
	if _, ok := a.Services[0].Environment["REMOVE"]; ok {
		t.Error("deleted env var came back")
	}
	if len(a.Services[0].Volumes) != 1 {
		t.Errorf("deleted volume came back: %v", a.Services[0].Volumes)
	}
}

func TestCaddyfileHashRoundTrip(t *testing.T) {
	am := newTestManager(t)
	createTestApp(t, am, &App{
		Name:     "demo",
		Domain:   "demo.example.com",
		Port:     3000,
		Services: []Service{{Name: "worker", Image: "busybox"}, {Name: "web", Image: "nginx"}},
	})

	if err := am.CreateDefaultCaddyfile("demo"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(am.AppsDir, "demo", "Caddyfile")
	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "demo.example.com {") || !strings.Contains(content, "reverse_proxy demo-web:3000") {
		t.Errorf("unexpected Caddyfile:\n%s", content)
	}

	changed, err := am.DetectCaddyfileChanges("demo")
	if err != nil || changed {
		t.Errorf("fresh Caddyfile detected as modified (changed=%v, err=%v)", changed, err)
	}

	if err := os.WriteFile(path, append(data, []byte("# manual edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := am.DetectCaddyfileChanges("demo"); !changed {
		t.Error("manual edit not detected")
	}
}

func TestHTTPServiceName(t *testing.T) {
	a := &App{Services: []Service{{Name: "api"}, {Name: "web"}}}
	if got := a.HTTPServiceName(); got != "web" {
		t.Errorf("got %q, want web", got)
	}
	a.HttpService = "api"
	if got := a.HTTPServiceName(); got != "api" {
		t.Errorf("got %q, want api", got)
	}
	a.HttpService = "missing"
	if got := a.HTTPServiceName(); got != "web" {
		t.Errorf("got %q, want web", got)
	}
}

func TestPublishedPortsSurviveRegeneration(t *testing.T) {
	am := newTestManager(t)
	createTestApp(t, am, &App{
		Name:     "demo",
		Domain:   "demo.example.com",
		Services: []Service{{Name: "db", Image: "postgres", ExtraPorts: []string{"5433:5432", "127.0.0.1:9000:9000"}}},
	})

	for i := 0; i < 2; i++ {
		a, err := am.LoadApp("demo")
		if err != nil {
			t.Fatal(err)
		}
		if got := a.Services[0].ExtraPorts; len(got) != 2 || got[0] != "5433:5432" || got[1] != "127.0.0.1:9000:9000" {
			t.Fatalf("round %d: ports = %v", i, got)
		}
		if err := am.SaveApp(a); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTopLevelKeysAndPreserve(t *testing.T) {
	am := newTestManager(t)
	createTestApp(t, am, &App{
		Name:     "demo",
		Domain:   "demo.example.com",
		Services: []Service{{Name: "db", Image: "postgres", Volumes: []string{"pgdata:/var/lib/postgresql/data"}}},
	})
	appDir := filepath.Join(am.AppsDir, "demo")
	composePath := filepath.Join(appDir, "docker-compose.yml")
	dm := docker.NewManager("")

	if changed, err := dm.DetectManualChanges(appDir); err != nil || changed {
		t.Fatalf("fresh compose reported as changed (%v, %v)", changed, err)
	}

	// Manual edit: named volume + custom service field
	data, _ := os.ReadFile(composePath)
	edited := strings.Replace(string(data), "    db:\n", "    db:\n        restart: always\n", 1) + "volumes:\n    pgdata: {}\n"
	if err := os.WriteFile(composePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := dm.DetectManualChanges(appDir); !changed {
		t.Fatal("manual edit not detected")
	}
	if err := dm.AcceptManualChanges(appDir); err != nil {
		t.Fatal(err)
	}
	if changed, _ := dm.DetectManualChanges(appDir); changed {
		t.Fatal("preserved compose still reported as changed")
	}

	// Regeneration keeps both customizations
	a, err := am.LoadApp("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := am.SaveApp(a); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(composePath)
	if !strings.Contains(string(data), "pgdata: {}") || !strings.Contains(string(data), "restart: always") {
		t.Errorf("customizations lost after regeneration:\n%s", data)
	}
	if changed, _ := dm.DetectManualChanges(appDir); changed {
		t.Error("regenerated compose reported as changed")
	}
}
