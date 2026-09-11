package console

import (
	"strings"
	"testing"
)

func TestClassifyTier(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Tier
	}{
		{"read only", "gettime", TierNormal},
		{"read only with args", "listplayers", TierNormal},
		{"version", "version", TierNormal},
		{"mutating", "settime 1 13 37", TierMutating},
		{"kick is mutating", "kick Noman being rude", TierMutating},
		{"ban is mutating", "ban add Noman 1 day", TierMutating},
		{"shutdown is destructive", "shutdown", TierDestructive},
		{"killall is destructive", "killall", TierDestructive},
		{"kickall is destructive", "kickall", TierDestructive},
		{"regionreset is destructive", "regionreset 1 1", TierDestructive},
		{"worldchunkreset is destructive", "worldchunkreset", TierDestructive},
		// The server's own command names are inconsistently cased, so matching
		// must not depend on it.
		{"uppercase", "SHUTDOWN", TierDestructive},
		{"mixed case", "ShutDown", TierDestructive},
		{"leading whitespace", "   shutdown", TierDestructive},
		{"tab separated args", "settime\t1 13 37", TierMutating},
		{"unknown command", "somemodcommand arg", TierNormal},
		{"empty", "", TierNormal},
		// A command merely containing a dangerous word is not dangerous.
		{"substring is not a match", "say shutdown is coming", TierMutating},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTier(tt.line); got != tt.want {
				t.Errorf("ClassifyTier(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestFirstWord(t *testing.T) {
	tests := []struct{ in, want string }{
		{"gettime", "gettime"},
		{"settime 1 13 37", "settime"},
		{"  spaced   out  ", "spaced"},
		{"tab\tseparated", "tab"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := FirstWord(tt.in); got != tt.want {
				t.Errorf("FirstWord(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr string
	}{
		{name: "ordinary command", line: "gettime"},
		{name: "with arguments", line: `say "hello there"`},
		{name: "quotes are allowed", line: `kick "Some Name" reason`},
		{name: "empty", line: "", wantErr: "enter a command"},
		{name: "whitespace only", line: "   \t ", wantErr: "enter a command"},
		// A newline would let one submission smuggle in a second command that
		// never appeared in the confirmation dialog or the history.
		{name: "embedded newline", line: "gettime\nshutdown", wantErr: "single line"},
		{name: "carriage return", line: "gettime\rshutdown", wantErr: "single line"},
		{name: "null byte", line: "gettime\x00", wantErr: "null byte"},
		{name: "too long", line: strings.Repeat("a", 2000), wantErr: "limit is"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.line)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate(%q) = %v, want nil", tt.line, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate(%q) succeeded, want an error", tt.line)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestIsDestructiveAllowed(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		allowed bool
		want    bool
	}{
		{"destructive permitted when allowed", "shutdown", true, true},
		{"destructive blocked when not allowed", "shutdown", false, false},
		{"mutating always permitted", "settime 1 0 0", false, true},
		{"normal always permitted", "gettime", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDestructiveAllowed(tt.line, tt.allowed); got != tt.want {
				t.Errorf("IsDestructiveAllowed(%q, %v) = %v, want %v",
					tt.line, tt.allowed, got, tt.want)
			}
		})
	}
}
