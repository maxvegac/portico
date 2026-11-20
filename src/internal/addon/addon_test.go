package addon

import "testing"

func TestVersionSelection(t *testing.T) {
	def := &Definition{Versions: map[string]VersionConfig{
		"5.7": {Image: "mysql:5.7"}, "8.4.7": {Image: "mysql:8.4.7"}, "9.5.0": {Image: "mysql:9.5.0"}, "10": {Image: "x:10"},
	}}
	want := []string{"10", "9.5.0", "8.4.7", "5.7"}
	got := def.GetAvailableVersions()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("versions = %v, want %v", got, want)
		}
	}
	for i := 0; i < 20; i++ { // map order is random: must be stable
		cfg, err := def.GetVersionConfig("")
		if err != nil || cfg.Image != "x:10" {
			t.Fatalf("default version = %+v, %v", cfg, err)
		}
	}
	if _, err := def.GetVersionConfig("1"); err == nil {
		t.Error("unknown version accepted")
	}
}

func TestEmbeddedDefinitionsParse(t *testing.T) {
	am := NewManager(t.TempDir(), t.TempDir())
	for _, name := range []string{"postgresql", "mysql", "mariadb", "mongodb", "redis", "valkey"} {
		def, err := am.LoadDefinition(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if def.LatestVersion() == "" {
			t.Errorf("%s has no versions", name)
		}
		for v, cfg := range def.Versions {
			if cfg.Image == "" || len(cfg.Ports) == 0 {
				t.Errorf("%s %s: missing image or ports", name, v)
			}
		}
	}
}
