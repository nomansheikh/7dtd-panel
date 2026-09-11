package sdtd

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TypedValue is one entry from an endpoint that returns a list of
// name/type/value triples: /api/serverinfo, /api/gameprefs and
// /api/gamestats all use this shape.
//
// The spec models these as a four-way anyOf over int, float, bool and string
// variants, which the generator turns into an opaque json.RawMessage with
// As*/From* accessors — unwieldy for what is really one field with a type
// tag. Decoding into this struct instead keeps the call sites readable.
type TypedValue struct {
	Name string          `json:"name"`
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"value"`
	// Default is present on /api/gameprefs and absent elsewhere.
	Default json.RawMessage `json:"default,omitempty"`
}

// String renders the value for display, regardless of its declared type.
func (v TypedValue) String() string {
	var s string
	if err := json.Unmarshal(v.Raw, &s); err == nil {
		return s
	}
	return string(v.Raw)
}

// Int returns the value as an integer, reporting whether it was one.
func (v TypedValue) Int() (int64, bool) {
	var n int64
	if err := json.Unmarshal(v.Raw, &n); err == nil {
		return n, true
	}
	// Some numeric fields arrive quoted; uptime in the log is the known case.
	var s string
	if err := json.Unmarshal(v.Raw, &s); err == nil {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// Float returns the value as a float, reporting whether it was numeric.
func (v TypedValue) Float() (float64, bool) {
	var f float64
	if err := json.Unmarshal(v.Raw, &f); err == nil {
		return f, true
	}
	return 0, false
}

// Bool returns the value as a boolean, reporting whether it was one.
func (v TypedValue) Bool() (bool, bool) {
	var b bool
	if err := json.Unmarshal(v.Raw, &b); err == nil {
		return b, true
	}
	return false, false
}

// ValueSet is an ordered list of TypedValues with name lookup.
type ValueSet struct {
	Values []TypedValue
	index  map[string]int
}

// NewValueSet indexes values by name. Later duplicates win, matching what a
// caller reading the list top to bottom would conclude.
func NewValueSet(values []TypedValue) ValueSet {
	index := make(map[string]int, len(values))
	for i, v := range values {
		index[v.Name] = i
	}
	return ValueSet{Values: values, index: index}
}

// Get returns the named value.
func (s ValueSet) Get(name string) (TypedValue, bool) {
	i, ok := s.index[name]
	if !ok {
		return TypedValue{}, false
	}
	return s.Values[i], true
}

// Str returns the named value as a string, or "" when absent.
func (s ValueSet) Str(name string) string {
	v, ok := s.Get(name)
	if !ok {
		return ""
	}
	return v.String()
}

// Int returns the named value as an integer, or 0 when absent or not numeric.
func (s ValueSet) Int(name string) int64 {
	v, ok := s.Get(name)
	if !ok {
		return 0
	}
	n, _ := v.Int()
	return n
}

// Bool returns the named value as a boolean, or false when absent.
func (s ValueSet) Bool(name string) bool {
	v, ok := s.Get(name)
	if !ok {
		return false
	}
	b, _ := v.Bool()
	return b
}

// Len reports how many values the set holds.
func (s ValueSet) Len() int { return len(s.Values) }

func (s ValueSet) GoString() string {
	return fmt.Sprintf("ValueSet(%d values)", len(s.Values))
}
