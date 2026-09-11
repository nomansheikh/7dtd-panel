package sdtd

import (
	"encoding/json"
	"testing"
)

func tv(name, typ, rawValue string) TypedValue {
	return TypedValue{Name: name, Type: typ, Raw: json.RawMessage(rawValue)}
}

func TestTypedValueAccessors(t *testing.T) {
	tests := []struct {
		name      string
		value     TypedValue
		wantStr   string
		wantInt   int64
		intOK     bool
		wantFloat float64
		floatOK   bool
		wantBool  bool
		boolOK    bool
	}{
		{
			name:    "string",
			value:   tv("GameWorld", "string", `"Navezgane"`),
			wantStr: "Navezgane",
		},
		{
			name:      "integer",
			value:     tv("MaxPlayers", "int", `8`),
			wantStr:   "8",
			wantInt:   8,
			intOK:     true,
			wantFloat: 8,
			floatOK:   true,
		},
		{
			name:      "negative integer",
			value:     tv("GameDifficulty", "int", `-1`),
			wantStr:   "-1",
			wantInt:   -1,
			intOK:     true,
			wantFloat: -1,
			floatOK:   true,
		},
		{
			name:      "float",
			value:     tv("RangedDamage", "float", `1.5`),
			wantStr:   "1.5",
			wantFloat: 1.5,
			floatOK:   true,
		},
		{
			name:     "bool true",
			value:    tv("EnableMapRendering", "bool", `true`),
			wantStr:  "true",
			wantBool: true,
			boolOK:   true,
		},
		{
			name:    "bool false",
			value:   tv("BuildCreate", "bool", `false`),
			wantStr: "false",
			boolOK:  true,
		},
		{
			// The log's uptime field is delivered as a quoted number, and it is
			// the only uptime source in the entire API, so this must work.
			name:      "quoted number is still an integer",
			value:     tv("uptime", "string", `"41831"`),
			wantStr:   "41831",
			wantInt:   41831,
			intOK:     true,
			wantFloat: 0,
			floatOK:   false,
		},
		{
			name:    "empty string",
			value:   tv("ServerWebsiteURL", "string", `""`),
			wantStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.String(); got != tt.wantStr {
				t.Errorf("String() = %q, want %q", got, tt.wantStr)
			}
			if got, ok := tt.value.Int(); ok != tt.intOK || got != tt.wantInt {
				t.Errorf("Int() = %d, %v; want %d, %v", got, ok, tt.wantInt, tt.intOK)
			}
			if got, ok := tt.value.Float(); ok != tt.floatOK || got != tt.wantFloat {
				t.Errorf("Float() = %v, %v; want %v, %v", got, ok, tt.wantFloat, tt.floatOK)
			}
			if got, ok := tt.value.Bool(); ok != tt.boolOK || got != tt.wantBool {
				t.Errorf("Bool() = %v, %v; want %v, %v", got, ok, tt.wantBool, tt.boolOK)
			}
		})
	}
}

func TestValueSetLookup(t *testing.T) {
	set := NewValueSet([]TypedValue{
		tv("GameType", "string", `"7DTD"`),
		tv("MaxPlayers", "int", `8`),
		tv("EnableMapRendering", "bool", `false`),
	})

	if set.Len() != 3 {
		t.Errorf("Len = %d, want 3", set.Len())
	}
	if got := set.Str("GameType"); got != "7DTD" {
		t.Errorf("Str = %q, want 7DTD", got)
	}
	if got := set.Int("MaxPlayers"); got != 8 {
		t.Errorf("Int = %d, want 8", got)
	}
	if got := set.Bool("EnableMapRendering"); got != false {
		t.Errorf("Bool = %v, want false", got)
	}

	t.Run("missing names are zero rather than panicking", func(t *testing.T) {
		if _, ok := set.Get("Nope"); ok {
			t.Error("Get reported a missing name as present")
		}
		if set.Str("Nope") != "" || set.Int("Nope") != 0 || set.Bool("Nope") {
			t.Error("accessors should return zero values for a missing name")
		}
	})

	t.Run("ordering is preserved", func(t *testing.T) {
		if set.Values[0].Name != "GameType" || set.Values[2].Name != "EnableMapRendering" {
			t.Error("NewValueSet should not reorder values")
		}
	})
}

func TestValueSetZeroValueIsUsable(t *testing.T) {
	// ServerInfo returns a zero ValueSet alongside an error, and callers that
	// ignore the error must not crash.
	var set ValueSet
	if set.Len() != 0 {
		t.Errorf("Len = %d, want 0", set.Len())
	}
	if _, ok := set.Get("anything"); ok {
		t.Error("zero ValueSet reported a value as present")
	}
	if set.Str("anything") != "" {
		t.Error("zero ValueSet returned a non-empty string")
	}
}
