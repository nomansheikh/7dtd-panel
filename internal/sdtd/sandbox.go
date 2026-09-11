package sdtd

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd/gen"
)

// SandboxChoice is one allowed value of a sandbox option.
//
// The game enumerates every sandbox option as a fixed set of choices with the
// same labels its own settings screen shows, so "150%" and "XP Only" come from
// the server rather than from a table the panel would have to keep in step.
type SandboxChoice struct {
	Index int             `json:"index"`
	Raw   json.RawMessage `json:"value"`
	Label string          `json:"localizedName"`
}

// String renders a choice's underlying value the way the console would take it.
func (c SandboxChoice) String() string {
	var b bool
	if err := json.Unmarshal(c.Raw, &b); err == nil {
		return strconv.FormatBool(b)
	}
	var s string
	if err := json.Unmarshal(c.Raw, &s); err == nil {
		return s
	}
	return string(c.Raw)
}

// SandboxDetails is the human-facing half of an option, present only when the
// request asks for detail.
type SandboxDetails struct {
	CategoryName  string `json:"categoryName"`
	InternalName  string `json:"internalName"`
	LocalizedName string `json:"localizedName"`
	Description   string `json:"description"`
}

// SandboxOption is one entry of the world's sandbox configuration.
type SandboxOption struct {
	Key          string          `json:"key"`
	Type         string          `json:"type"`
	ActiveValue  SandboxChoice   `json:"activeValue"`
	DefaultValue SandboxChoice   `json:"defaultValue"`
	ValueSet     []SandboxChoice `json:"valueSet"`
	Details      *SandboxDetails `json:"details"`
}

// SandboxSettings is the world's sandbox configuration: its code plus every
// option.
type SandboxSettings struct {
	Code    string          `json:"code"`
	Options []SandboxOption `json:"options"`
}

// SandboxSettings fetches the world's sandbox options with their descriptions
// and allowed values.
//
// Read-only, and not only because the REST endpoint has no write: the console
// has no setsandboxoptions either, and setgamepref refuses these keys outright
// ("Error parsing parameter: RangedDamage"). They are fixed when the world is
// created. The panel shows them so an operator can read the world's rules
// without joining the game, and says plainly that they cannot be changed here.
func (c *Client) SandboxSettings(ctx context.Context) (SandboxSettings, error) {
	detailed := true
	params := &gen.SandboxsettingsGetParams{Detailed: &detailed}
	return fetch[SandboxSettings](func() (*http.Response, error) {
		return c.gen.SandboxsettingsGet(ctx, params)
	})
}
