package settings

/*
Which preferences the server reads once, when it starts.

The panel's first rule for "can this be changed" was whether getgamepref
reports the preference back, on the reasoning that a value the server will not
read out is one it is not using. That rule is not enough, and EnableMapRendering
is the proof: the console accepts `setgamepref EnableMapRendering true`, answers
"EnableMapRendering set to True", and the map renderer stays off, because the
renderer is built at startup from serverconfig.xml and never consulted again.
The game says so itself — `rendermap` answers "Renderer not enabled" and
`enablerendering` carries the note "This command can only turn the renderer off,
it can not turn it on if it is not enabled in the serverconfig!".

Worse, setgamepref does not write serverconfig.xml, so a change to one of these
is inert twice over: nothing happens now, and nothing survives a restart. An
operator who is told the change worked has been told something false.

Only EnableMapRendering has been demonstrated against a live server. The rest
are grouped by the subsystem they configure — the web and telnet listeners, the
world's own identity, the network and anticheat stack — all of which are built
during startup. Where a preference was uncertain it was left out: wrongly
refusing a change an operator could have made is its own kind of wrong.
*/

// startupOnlyNames are read from the server's config file at startup. Changing
// one through the console is accepted and does nothing.
var startupOnlyNames = map[string]struct{}{
	// The map renderer. Demonstrated: the value changes, the renderer does not.
	"EnableMapRendering": {},

	// The web dashboard the panel itself talks to, and the telnet listener.
	// Both are sockets opened during startup. Turning the dashboard off here
	// would also be the panel cutting its own line.
	"WebDashboardEnabled": {},
	"WebDashboardPort":    {},
	"WebDashboardUrl":     {},
	"TelnetEnabled":       {},
	"TelnetPort":          {},
	"TelnetPassword":      {},

	// The terminal window, which a headless server decides about at startup.
	"TerminalWindowEnabled": {},

	// The world's own identity. These are not merely inert: they describe
	// which world is loaded and how it was generated, and the server would act
	// on a changed value the next time it started rather than now.
	"GameWorld":                {},
	"GameName":                 {},
	"WorldGenSeed":             {},
	"WorldGenSize":             {},
	"UserDataFolder":           {},
	"PersistentPlayerProfiles": {},
	"AdminFileName":            {},

	// The network and anticheat stack, bound and initialised before the world
	// finishes loading.
	"ServerPort":                     {},
	"ServerVisibility":               {},
	"ServerAllowCrossplay":           {},
	"ServerDisabledNetworkProtocols": {},
	"EACEnabled":                     {},
	"IgnoreEOSSanctions":             {},

	// The dynamic mesh system, built with the world.
	"DynamicMeshEnabled":  {},
	"MaxQueuedMeshLayers": {},
}

// StartupOnly reports whether a preference is one the server reads only at
// startup, and so cannot usefully be changed while it is running.
func StartupOnly(name string) bool {
	_, ok := startupOnlyNames[name]
	return ok
}
