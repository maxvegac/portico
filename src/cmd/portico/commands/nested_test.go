package commands

import (
	"reflect"
	"testing"
)

func TestNestedPositionalsFrom(t *testing.T) {
	subs := map[string]bool{"add": true, "del": true, "delete": true, "list": true}

	tests := []struct {
		args []string
		want []string
	}{
		// Service omitted: arguments after the subcommand must not be taken as the service
		{[]string{"ports", "my-app", "add", "8080", "3000"}, []string{"my-app"}},
		{[]string{"ports", "my-app", "web", "add", "8080", "3000"}, []string{"my-app", "web"}},
		{[]string{"env", "my-app", "add", "KEY", "VALUE"}, []string{"my-app"}},
		{[]string{"storage", "my-app", "add", "/host", "/container"}, []string{"my-app"}},
		// Flags are skipped, empty arguments don't panic
		{[]string{"env", "--verbose", "my-app", "list"}, []string{"my-app"}},
		{[]string{"set", "", "list"}, []string{""}},
		{[]string{"other"}, nil},
	}

	for _, tt := range tests {
		got := nestedPositionalsFrom(tt.args, tt.args[0], subs)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("nestedPositionalsFrom(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestAddonsPositionals(t *testing.T) {
	got := nestedPositionalsFrom([]string{"addons", "my-app", "link", "my-pg", "--database", "x"}, "addons", addonActions)
	if !reflect.DeepEqual(got, []string{"my-app"}) {
		t.Errorf("got %q", got)
	}
	got = nestedPositionalsFrom([]string{"addons", "my-pg", "database", "create", "mydb"}, "addons", addonActions)
	if !reflect.DeepEqual(got, []string{"my-pg"}) {
		t.Errorf("got %q", got)
	}
}

func TestValidateDomain(t *testing.T) {
	for _, d := range []string{"example.com", "app.1-2-3-4.sslip.io", "*.example.com"} {
		if err := validateDomain(d); err != nil {
			t.Errorf("validateDomain(%q) = %v", d, err)
		}
	}
	for _, d := range []string{"", "example", "exa mple.com", "example.com {\n respond hacked\n}", "http://example.com"} {
		if err := validateDomain(d); err == nil {
			t.Errorf("validateDomain(%q) = nil, want error", d)
		}
	}
}
