package console

import "testing"

// Every command the panel sends has to land on a sensible tier. Anything the
// policy does not recognise falls through to normal, which silently skips the
// confirmation a state change deserves — so a command added to the panel and
// forgotten here is a real hole, not a cosmetic one.
func TestPanelCommandsAreTiered(t *testing.T) {
	for command, want := range map[string]Tier{
		"showinventory 1":              TierNormal,
		"listlandprotection summary":   TierNormal,
		"admin list":                   TierMutating,
		"whitelist list":               TierMutating,
		"mem":                          TierNormal,
		"version":                      TierNormal,
		"unlock 1":                     TierMutating,
		"overridemaxplayercount 16":    TierMutating,
		"removelandprotection Steam_1": TierDestructive,
		"shutdown":                     TierDestructive,
		"kickall":                      TierDestructive,
	} {
		if got := ClassifyTier(command); got != want {
			t.Errorf("ClassifyTier(%q) = %q, want %q", command, got, want)
		}
	}
}
