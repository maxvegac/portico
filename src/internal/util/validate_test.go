package util

import "testing"

func TestValidateAppName(t *testing.T) {
	valid := []string{"my-app", "app1", "a", "my_app", "facturacion-api"}
	invalid := []string{"", ".", "..", "../etc", "a/b", "My-App", "-app", "app-", "app name", "app.example"}

	for _, name := range valid {
		if err := ValidateAppName(name); err != nil {
			t.Errorf("ValidateAppName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if err := ValidateAppName(name); err == nil {
			t.Errorf("ValidateAppName(%q) = nil, want error", name)
		}
	}
}

func TestValidateSecretName(t *testing.T) {
	for _, name := range []string{"db_password", "API_KEY", "tls.crt"} {
		if err := ValidateSecretName(name); err != nil {
			t.Errorf("ValidateSecretName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", ".", "..", "../x", "a/b", ".hidden"} {
		if err := ValidateSecretName(name); err == nil {
			t.Errorf("ValidateSecretName(%q) = nil, want error", name)
		}
	}
}

func TestValidateDBIdentifier(t *testing.T) {
	for _, name := range []string{"mydb", "my_app", "_x1"} {
		if err := ValidateDBIdentifier(name); err != nil {
			t.Errorf("ValidateDBIdentifier(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "my-app", "1db", "x; DROP DATABASE y", "a'b", "a`b"} {
		if err := ValidateDBIdentifier(name); err == nil {
			t.Errorf("ValidateDBIdentifier(%q) = nil, want error", name)
		}
	}
}

func TestRandomSecret(t *testing.T) {
	a, err := RandomSecret(16)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := RandomSecret(16)
	if len(a) != 32 || a == b {
		t.Errorf("RandomSecret returned %q and %q", a, b)
	}
}

func TestValidateNewAppName(t *testing.T) {
	for _, name := range []string{"reverse-proxy", "portico", "addon", "addon-pg"} {
		if err := ValidateNewAppName(name); err == nil {
			t.Errorf("ValidateNewAppName(%q) = nil, want error", name)
		}
	}
	if err := ValidateNewAppName("my-app"); err != nil {
		t.Error(err)
	}
	if err := ValidateServiceName("web.v2"); err == nil {
		t.Error("service names with dots must be rejected (they form DNS aliases)")
	}
}
