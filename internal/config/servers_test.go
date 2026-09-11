package config

import (
	"strings"
	"testing"
)

func TestSingleServerFormStillWorks(t *testing.T) {
	// An existing deployment must keep working untouched.
	c, err := Load(env(minimal()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(c.Servers))
	}
	g := c.Default()
	if g.ID != "default" {
		t.Errorf("id = %q, want default", g.ID)
	}
	if g.Host != "10.0.0.5" {
		t.Errorf("host = %q", g.Host)
	}
	// Without an explicit name the host is more useful than "default".
	if g.Name != "10.0.0.5" {
		t.Errorf("name = %q, want the host", g.Name)
	}
}

func TestMultipleServers(t *testing.T) {
	vars := map[string]string{
		"PANEL_ADMIN_PASSWORD":         "correct-horse",
		"SDTD_SERVERS":                 "main, backup",
		"SDTD_MAIN_NAME":               "Main survival",
		"SDTD_MAIN_HOST":               "10.0.0.5",
		"SDTD_MAIN_API_TOKEN_NAME":     "panel",
		"SDTD_MAIN_API_TOKEN_SECRET":   "secret-one",
		"SDTD_BACKUP_HOST":             "10.0.0.6",
		"SDTD_BACKUP_API_PORT":         "8100",
		"SDTD_BACKUP_API_SCHEME":       "https",
		"SDTD_BACKUP_API_TOKEN_NAME":   "panel2",
		"SDTD_BACKUP_API_TOKEN_SECRET": "secret-two",
	}
	c, err := Load(env(vars))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(c.Servers))
	}

	main := c.Servers[0]
	if main.ID != "main" || main.Name != "Main survival" {
		t.Errorf("first server = %+v", main)
	}
	if got := main.BaseURL(); got != "http://10.0.0.5:8080" {
		t.Errorf("main base URL = %q", got)
	}

	backup := c.Servers[1]
	if backup.ID != "backup" {
		t.Errorf("second id = %q", backup.ID)
	}
	// Without a name, the ID is the label.
	if backup.Name != "backup" {
		t.Errorf("backup name = %q, want backup", backup.Name)
	}
	if got := backup.BaseURL(); got != "https://10.0.0.6:8100" {
		t.Errorf("backup base URL = %q", got)
	}
	if backup.TokenSecret != "secret-two" {
		t.Error("each server must carry its own credentials")
	}

	// Order matters: the first listed is what the UI opens on.
	if c.Default().ID != "main" {
		t.Errorf("default = %q, want main", c.Default().ID)
	}
}

func TestServerLookup(t *testing.T) {
	c, err := Load(env(minimal()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := c.Server("default"); !ok {
		t.Error("Server(default) should be found")
	}
	if _, ok := c.Server("nope"); ok {
		t.Error("Server(nope) should not be found")
	}
}

func TestServerIDsAreNormalisedAndValidated(t *testing.T) {
	tests := []struct {
		name    string
		list    string
		wantErr string
	}{
		{name: "upper case is lowered", list: "MAIN"},
		{name: "dashes are allowed", list: "eu-west"},
		{name: "spaces are trimmed", list: " main , backup "},
		{name: "underscores rejected", list: "eu_west", wantErr: "lower case letters"},
		{name: "dots rejected", list: "eu.west", wantErr: "lower case letters"},
		{name: "slashes rejected", list: "a/b", wantErr: "lower case letters"},
		{name: "leading dash rejected", list: "-main", wantErr: "lower case letters"},
		{name: "duplicates rejected", list: "main,main", wantErr: "more than once"},
		{name: "empty list rejected", list: " , ", wantErr: "no usable server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := strings.ToUpper(strings.ReplaceAll(
				strings.TrimSpace(strings.Split(tt.list, ",")[0]), "-", "_"))
			vars := map[string]string{
				"PANEL_ADMIN_PASSWORD":             "correct-horse",
				"SDTD_SERVERS":                     tt.list,
				"SDTD_" + id + "_HOST":             "10.0.0.5",
				"SDTD_" + id + "_API_TOKEN_NAME":   "panel",
				"SDTD_" + id + "_API_TOKEN_SECRET": "secret",
				"SDTD_BACKUP_HOST":                 "10.0.0.6",
				"SDTD_BACKUP_API_TOKEN_NAME":       "panel",
				"SDTD_BACKUP_API_TOKEN_SECRET":     "secret",
			}
			_, err := Load(env(vars))

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Load: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Load accepted %q", tt.list)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestMissingCredentialsAreReportedPerServer(t *testing.T) {
	// Naming which server is misconfigured matters once there is more than one.
	vars := map[string]string{
		"PANEL_ADMIN_PASSWORD":       "correct-horse",
		"SDTD_SERVERS":               "main,backup",
		"SDTD_MAIN_HOST":             "10.0.0.5",
		"SDTD_MAIN_API_TOKEN_NAME":   "panel",
		"SDTD_MAIN_API_TOKEN_SECRET": "secret",
		// backup is entirely absent
	}
	_, err := Load(env(vars))
	if err == nil {
		t.Fatal("Load succeeded with a server missing its settings")
	}
	for _, want := range []string{"SDTD_BACKUP_HOST", "SDTD_BACKUP_API_TOKEN_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %s:\n%v", want, err)
		}
	}
}

func TestRedactedListsServersWithoutSecrets(t *testing.T) {
	vars := map[string]string{
		"PANEL_ADMIN_PASSWORD":         "correct-horse",
		"SDTD_SERVERS":                 "main,backup",
		"SDTD_MAIN_HOST":               "10.0.0.5",
		"SDTD_MAIN_API_TOKEN_NAME":     "panel",
		"SDTD_MAIN_API_TOKEN_SECRET":   "super-secret-one",
		"SDTD_BACKUP_HOST":             "10.0.0.6",
		"SDTD_BACKUP_API_TOKEN_NAME":   "panel",
		"SDTD_BACKUP_API_TOKEN_SECRET": "super-secret-two",
	}
	c, err := Load(env(vars))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	rendered := ""
	for k, v := range c.Redacted() {
		rendered += k + "=" + toString(v) + ";"
	}
	for _, secret := range []string{"super-secret-one", "super-secret-two"} {
		if strings.Contains(rendered, secret) {
			t.Errorf("Redacted leaked %q", secret)
		}
	}
	for _, want := range []string{"main", "backup"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("Redacted should list server %q", want)
		}
	}
}
