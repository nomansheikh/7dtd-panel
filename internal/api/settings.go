package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/settings"
)

// settingChoice is one allowed value of a setting, with the label the game's
// own settings screen uses for it.
type settingChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type settingRow struct {
	// Name is the raw preference name, shown small so an operator can still
	// search for it or use it in the console.
	Name string `json:"name"`
	// Label is the readable form the UI leads with.
	Label string `json:"label"`
	// Description is the game's own explanation, present for the 165 settings
	// that appear on the in-game sandbox screen.
	Description string `json:"description,omitempty"`
	Group       string `json:"group"`
	// Type is bool, int, float or string, which is what decides the control
	// when there are no choices to pick from.
	Type    string `json:"type"`
	Value   any    `json:"value"`
	Default any    `json:"default"`
	// ValueLabel and DefaultLabel are the game's words for the current and
	// default values, so an operator reads "XP Only" rather than "1".
	ValueLabel   string `json:"valueLabel,omitempty"`
	DefaultLabel string `json:"defaultLabel,omitempty"`
	// Changed is true when the value differs from the server's default.
	Changed bool `json:"changed"`
	// Editable is false for settings the running server refuses to change.
	Editable bool `json:"editable"`
	// ReadOnlyReason explains why, rather than leaving a disabled control
	// unexplained.
	ReadOnlyReason string `json:"readOnlyReason,omitempty"`
	// Choices, when present, make this a picker rather than a free field.
	Choices []settingChoice `json:"choices,omitempty"`
}

type settingsGroup struct {
	Name     string       `json:"name"`
	Settings []settingRow `json:"settings"`
}

type settingsSection struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Groups      []settingsGroup `json:"groups"`
	Total       int             `json:"total"`
}

const (
	worldReadOnly = "This is part of the world's sandbox configuration, fixed when the " +
		"world was created. The server has no command to change it while it is running."
	clientReadOnly = "This preference belongs to the game client. It is in the same list " +
		"because client and server share one set of names, and it does nothing on a server."
	startupOnly = "The server does not report this one back, so a change here could not be " +
		"confirmed. These are the settings it reads once at startup, such as the telnet " +
		"port: change them in the server's own config and restart."
)

// handleSettings lists every game preference, grouped and labelled.
//
// The raw list is 287 undifferentiated name/type/value entries, of which about
// 180 are the game client's own graphics and audio options. The work here is
// turning that into something operable: the game's sandbox screen supplies
// descriptions and the exact set of allowed values for 165 settings, so most
// of them become a picker with the same wording that appears in game.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	prefs, err := srv.Client.GamePrefs(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not load the settings")
		return
	}

	// The sandbox call is the source of every description and label, but the
	// page is still worth showing without it.
	sandbox, sandboxErr := srv.Client.SandboxSettings(r.Context())
	if sandboxErr != nil {
		s.log.Warn("could not load the sandbox settings", "error", sandboxErr)
	}

	// /api/gameprefs reports what the server started with, so every value it
	// gives is overlaid with what the console says is running.
	live, liveErr := srv.Client.GamePrefsLive(r.Context())
	if liveErr != nil {
		s.log.Warn("could not read the live preferences", "error", liveErr)
	}
	prefs = overlayLive(prefs, live)

	sections := []settingsSection{
		worldSection(prefs, sandbox.Options, live),
		serverSection(prefs, sandbox.Options, live),
		clientSection(prefs),
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"sandboxCode": sandbox.Code,
		"sections":    sections,
	})
}

// settable reports whether the running server will accept a change to a
// preference and let it be read back.
//
// getgamepref lists about 153 of the 287 preferences. setgamepref will accept
// the others and answer "set to x", but getgamepref then reports nothing for
// them, and they are the ones the server read once at startup — the telnet
// port, the admin file name. Offering a control that cannot be verified and
// would not take effect is worse than saying so.
//
// When the console could not be read at all, everything stays editable: a
// momentary failure should not grey out the page, and the write would report
// its own error.
func settable(name string, live map[string]string) bool {
	if len(live) == 0 {
		return true
	}
	_, ok := live[name]
	return ok
}

// overlayLive replaces each preference's value with the one the console
// reports, keeping the type and default from the REST reply.
//
// The console answers in plain text, so a value is re-encoded according to the
// type the preference declares. A value that will not parse as its declared
// type is left alone rather than guessed at.
func overlayLive(prefs sdtd.ValueSet, live map[string]string) sdtd.ValueSet {
	if len(live) == 0 {
		return prefs
	}
	values := make([]sdtd.TypedValue, 0, len(prefs.Values))
	for _, pref := range prefs.Values {
		if text, ok := live[pref.Name]; ok {
			if raw, ok := encodeAs(pref.Type, text); ok {
				pref.Raw = raw
			}
		}
		values = append(values, pref)
	}
	return sdtd.NewValueSet(values)
}

// encodeAs renders a console value as JSON of the declared type.
func encodeAs(declaredType, text string) (json.RawMessage, bool) {
	switch strings.ToLower(declaredType) {
	case "bool":
		b, err := strconv.ParseBool(text)
		if err != nil {
			return nil, false
		}
		return json.RawMessage(strconv.FormatBool(b)), true
	case "int", "float":
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return nil, false
		}
		return json.RawMessage(text), true
	default:
		encoded, err := json.Marshal(text)
		if err != nil {
			return nil, false
		}
		return encoded, true
	}
}

// worldSection is the in-game sandbox screen: the game's categories, its
// wording and its lists of allowed values.
func worldSection(prefs sdtd.ValueSet, options []sdtd.SandboxOption, live map[string]string) settingsSection {
	section := settingsSection{
		ID:    "world",
		Title: "World rules",
		Description: "The settings the game's own sandbox screen shows, in its own words. " +
			"Most were fixed when the world was created and can only be changed by " +
			"starting a new one.",
	}

	// First-seen order, which is the order the in-game screen uses.
	var order []string
	byCategory := map[string][]settingRow{}

	for _, opt := range options {
		row := settingRow{
			Name:           opt.Key,
			Label:          settings.Label(opt.Key),
			Type:           strings.ToLower(opt.Type),
			Value:          decodeTyped(opt.ActiveValue.Raw),
			Default:        decodeTyped(opt.DefaultValue.Raw),
			ValueLabel:     opt.ActiveValue.Label,
			DefaultLabel:   opt.DefaultValue.Label,
			ReadOnlyReason: worldReadOnly,
		}
		category := "Misc"
		if opt.Details != nil {
			category = opt.Details.CategoryName
			row.Description = opt.Details.Description
			if opt.Details.LocalizedName != "" {
				row.Label = opt.Details.LocalizedName
			}
		}
		row.Group = category

		// A sandbox option that is also a preference can be set with
		// setgamepref, so it gets real controls. The preference is the
		// authority on the value: the two disagree where the sandbox screen
		// shows a percentage and the preference holds a whole number.
		if pref, ok := prefs.Get(opt.Key); ok {
			row.Editable = settable(opt.Key, live)
			row.ReadOnlyReason = ""
			if !row.Editable {
				row.ReadOnlyReason = startupOnly
			}
			row.Type = strings.ToLower(pref.Type)
			row.Value = decodeTyped(pref.Raw)
			row.Default = decodeTyped(pref.Default)
			row.Choices = choicesFor(opt, pref.Raw)
			row.ValueLabel = labelFor(row.Choices, pref.Raw)
			row.DefaultLabel = labelFor(row.Choices, pref.Default)

			// The preference's own default is sometimes on a different scale
			// from the choices: BloodMoonWarning offers Morning, Midday and
			// Evening, and reports its default as 8. The sandbox default is
			// the one that means something next to the choices.
			if len(row.Choices) > 0 && row.DefaultLabel == "" {
				row.Default = decodeTyped(opt.DefaultValue.Raw)
				row.DefaultLabel = labelFor(row.Choices, opt.DefaultValue.Raw)
				if row.DefaultLabel == "" {
					// Neither default maps onto the choices; claiming one
					// would only mislead.
					row.Default = nil
				}
			}
		}
		row.Changed = row.Default != nil && row.Value != row.Default

		if _, seen := byCategory[category]; !seen {
			order = append(order, category)
		}
		byCategory[category] = append(byCategory[category], row)
		section.Total++
	}

	for _, category := range order {
		section.Groups = append(section.Groups, settingsGroup{
			Name: category, Settings: byCategory[category],
		})
	}
	return section
}

// serverSection holds the preferences that run the server itself and are not
// already covered by the world rules.
func serverSection(prefs sdtd.ValueSet, options []sdtd.SandboxOption, live map[string]string) settingsSection {
	covered := make(map[string]struct{}, len(options))
	for _, opt := range options {
		covered[opt.Key] = struct{}{}
	}

	section := settingsSection{
		ID:    "server",
		Title: "Server settings",
		Description: "Preferences the running server will accept a change to. Changes take " +
			"effect immediately and are lost when the server restarts, because the game " +
			"does not write them back to its config file.",
	}

	byGroup := map[settings.Group][]settingRow{}
	for _, pref := range prefs.Values {
		if _, ok := covered[pref.Name]; ok {
			continue
		}
		if settings.ClientOnly(pref.Name) {
			continue
		}
		row := prefRow(pref)
		if row.Editable = settable(pref.Name, live); !row.Editable {
			row.ReadOnlyReason = startupOnly
		}
		byGroup[settings.Group(row.Group)] = append(byGroup[settings.Group(row.Group)], row)
		section.Total++
	}

	for _, g := range settings.Order {
		rows := byGroup[g]
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(a, b int) bool { return rows[a].Label < rows[b].Label })
		section.Groups = append(section.Groups, settingsGroup{Name: string(g), Settings: rows})
	}
	return section
}

// clientSection is the game client's own options, kept together and out of the
// way rather than dropped.
func clientSection(prefs sdtd.ValueSet) settingsSection {
	section := settingsSection{
		ID:    "client",
		Title: "Client preferences",
		Description: "Graphics, audio and controller options that arrive in the same list " +
			"because the client and the server share one set of preference names. They " +
			"have no effect on a dedicated server.",
	}

	var rows []settingRow
	for _, pref := range prefs.Values {
		if !settings.ClientOnly(pref.Name) {
			continue
		}
		row := prefRow(pref)
		row.ReadOnlyReason = clientReadOnly
		rows = append(rows, row)
		section.Total++
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a].Name < rows[b].Name })
	if len(rows) > 0 {
		section.Groups = []settingsGroup{{Name: "Client", Settings: rows}}
	}
	return section
}

func prefRow(pref sdtd.TypedValue) settingRow {
	row := settingRow{
		Name:    pref.Name,
		Label:   settings.Label(pref.Name),
		Group:   string(settings.GroupFor(pref.Name)),
		Type:    strings.ToLower(pref.Type),
		Value:   decodeTyped(pref.Raw),
		Default: decodeTyped(pref.Default),
	}
	row.Changed = row.Default != nil && row.Value != row.Default
	return row
}

// choicesFor turns a sandbox option's value set into pickable choices, but
// only when the preference's own value is one of them.
//
// The two sides do not always use the same scale: the sandbox screen holds
// XPMultiplier as the fraction 1 while the preference holds 100. Where every
// choice lands on a whole number once multiplied by a hundred and the
// preference's value is one of those, the percentage reading is the right one.
// Anything else falls back to a plain field, because offering choices that
// would write the wrong number is worse than offering none.
func choicesFor(opt sdtd.SandboxOption, current json.RawMessage) []settingChoice {
	if len(opt.ValueSet) == 0 || len(current) == 0 {
		return nil
	}
	if choices, ok := matchChoices(opt.ValueSet, current, 1); ok {
		return choices
	}
	choices, ok := matchChoices(opt.ValueSet, current, 100)
	if !ok {
		return nil
	}
	return choices
}

// matchChoices renders a value set at the given scale, reporting whether the
// preference's current value is among the results.
func matchChoices(set []sdtd.SandboxChoice, current json.RawMessage, scale float64) ([]settingChoice, bool) {
	matched := false
	choices := make([]settingChoice, 0, len(set))
	for _, c := range set {
		value := c.String()
		if scale != 1 {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, false
			}
			scaled := f * scale
			if scaled != math.Trunc(scaled) {
				return nil, false
			}
			value = strconv.FormatFloat(scaled, 'f', -1, 64)
		}
		if sameValue(json.RawMessage(value), current) {
			matched = true
		}
		choices = append(choices, settingChoice{Value: value, Label: c.Label})
	}
	return choices, matched
}

// labelFor finds the label a raw value has among the choices.
func labelFor(choices []settingChoice, raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	value := strings.Trim(string(raw), `"`)
	for _, c := range choices {
		if strings.EqualFold(c.Value, value) {
			return c.Label
		}
	}
	return ""
}

// sameValue compares two raw JSON values numerically where it can, so 1 and
// 1.0 are the same choice.
func sameValue(a, b json.RawMessage) bool {
	var af, bf float64
	if json.Unmarshal(a, &af) == nil && json.Unmarshal(b, &bf) == nil {
		return af == bf
	}
	return strings.EqualFold(strings.Trim(string(a), `"`), strings.Trim(string(b), `"`))
}

type updateSettingRequest struct {
	// Value is sent as a string; the game console takes everything as text and
	// the declared type decides how it is validated here.
	Value string `json:"value"`
}

// handleUpdateSetting changes one preference.
//
// setgamepref is the only write path: PUT on /api/gameprefs returns 405. The
// new value is read back through the console rather than trusted, because
// /api/gameprefs does not reflect runtime changes and would report the old one.
func (s *Server) handleUpdateSetting(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := r.PathValue("name")

	var req updateSettingRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}

	// Checked against the server's own list, so an unknown or misspelled name
	// is refused before it becomes a command.
	prefs, err := srv.Client.GamePrefs(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not load the settings")
		return
	}
	pref, found := prefs.Get(name)
	if !found {
		httpx.WriteError(w, http.StatusBadRequest,
			"no setting called "+name+" on this server", "UNKNOWN_SETTING")
		return
	}
	if settings.ClientOnly(name) {
		httpx.WriteError(w, http.StatusBadRequest,
			name+" belongs to the game client and does nothing on a server", "CLIENT_SETTING")
		return
	}

	live, liveErr := srv.Client.GamePrefsLive(r.Context())
	if liveErr != nil {
		s.log.Warn("could not read the live preferences", "error", liveErr)
	}
	if !settable(name, live) {
		httpx.WriteError(w, http.StatusBadRequest, startupOnly, "STARTUP_SETTING")
		return
	}

	value, err := normaliseValue(pref.Type, req.Value)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_VALUE")
		return
	}

	command, buildErr := console.SetGamePref(name, value)
	if buildErr != nil {
		httpx.WriteError(w, http.StatusBadRequest, buildErr.Error(), "INVALID_ARGUMENT")
		return
	}

	user, _ := UserFrom(r.Context())
	s.log.Info("setting changed", "username", user.Username, "setting", name, "value", value)

	result, err := srv.Client.Execute(r.Context(), command)
	if err != nil {
		s.writeGameError(w, err, "could not change the setting")
		return
	}
	// The console answers a rejected name or value with text and a 200, so the
	// reply has to be read rather than just the status.
	if strings.Contains(result.Result, "Error parsing parameter") {
		httpx.WriteError(w, http.StatusBadRequest,
			"the server would not accept that value for "+name, "REJECTED_BY_SERVER")
		return
	}

	// Read back the live value rather than echoing what was asked for.
	applied, readErr := srv.Client.ReadGamePref(r.Context(), name)
	if readErr != nil {
		s.log.Warn("could not read the setting back", "setting", name, "error", readErr)
		applied = value
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"name":    name,
		"value":   applied,
		"command": command,
		// Runtime only: the game writes nothing back to serverconfig.xml, so
		// this reverts when the server restarts.
		"persisted": false,
	})
}

// normaliseValue validates a submitted value against the preference's declared
// type and renders it the way the console expects.
func normaliseValue(declaredType, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	switch strings.ToLower(declaredType) {
	case "bool":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return "", errInvalid("must be true or false", value)
		}
		return strconv.FormatBool(b), nil
	case "int":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", errInvalid("must be a whole number", value)
		}
		return strconv.FormatInt(n, 10), nil
	case "float":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", errInvalid("must be a number", value)
		}
		return strconv.FormatFloat(f, 'g', -1, 64), nil
	default:
		if value == "" {
			return "", errInvalid("must not be empty", value)
		}
		// The console splits on spaces and has no documented quoting, so a
		// value containing one cannot be sent safely. A newline is worse: it
		// would append a second command nobody asked for.
		if strings.ContainsFunc(value, unicode.IsSpace) {
			return "", errInvalid("cannot contain spaces or line breaks, because the game console has no way to quote them", value)
		}
		return value, nil
	}
}

type valueError struct{ msg string }

func (e *valueError) Error() string { return e.msg }

func errInvalid(why, got string) error {
	return &valueError{"the value " + why + ", got " + strconv.Quote(got)}
}

// decodeTyped renders a raw JSON value as a Go value for the response, so the
// UI receives a real boolean or number rather than a string it must parse.
func decodeTyped(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
