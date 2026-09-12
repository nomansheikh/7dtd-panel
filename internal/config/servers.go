package config

import (
	"regexp"
	"strings"
)

// Multiple servers are configured with indexed environment variables, so the
// panel still needs no config file to mount:
//
//	SDTD_SERVERS=main,backup
//	SDTD_MAIN_HOST=10.0.0.5
//	SDTD_MAIN_API_TOKEN_NAME=panel
//	SDTD_MAIN_API_TOKEN_SECRET=...
//	SDTD_BACKUP_HOST=10.0.0.6
//	...
//
// The single-server form is still accepted and becomes one server with the ID
// "default", so an existing deployment keeps working untouched.

// idPattern is what a server ID may contain. It ends up in URLs, so it is kept
// deliberately narrow.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// envSuffix converts a server ID into the environment variable infix:
// "main" -> "MAIN", "eu-west" -> "EU_WEST".
func envSuffix(id string) string {
	return strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
}

// servers parses every configured game server.
func (p *parser) servers() []Game {
	list := strings.TrimSpace(p.getenv("SDTD_SERVERS"))
	if list == "" {
		// Nothing about a game server in the environment at all is now a
		// legitimate way to start: the panel boots with none and offers to add
		// one. But a half-filled environment is still an error — somebody who
		// set SDTD_API_PORT and misspelled SDTD_HOST meant to configure a
		// server, and silence would leave them staring at an empty panel
		// wondering why.
		if !p.anySingleServerVar() {
			return nil
		}
		return []Game{p.singleServer()}
	}

	var (
		out  []Game
		seen = map[string]bool{}
	)
	for _, raw := range strings.Split(list, ",") {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "" {
			continue
		}
		if !idPattern.MatchString(id) {
			p.errf("SDTD_SERVERS entry %q must be lower case letters, digits and dashes", id)
			continue
		}
		if seen[id] {
			p.errf("SDTD_SERVERS lists %q more than once", id)
			continue
		}
		seen[id] = true
		out = append(out, p.serverWithPrefix(id, "SDTD_"+envSuffix(id)+"_"))
	}

	if len(out) == 0 {
		p.errf("SDTD_SERVERS is set but lists no usable server names")
		// Keep Config.Default() safe for callers that ignore the error.
		return []Game{{ID: "default", Name: "default", Scheme: "http", Port: 8080}}
	}
	return out
}

// singleServer reads the original unprefixed variables.
// anySingleServerVar reports whether the environment says anything at all about
// a single game server.
func (p *parser) anySingleServerVar() bool {
	for _, name := range []string{
		"SDTD_NAME", "SDTD_HOST", "SDTD_API_PORT", "SDTD_API_SCHEME",
		"SDTD_API_TOKEN_NAME", "SDTD_API_TOKEN_SECRET",
	} {
		if strings.TrimSpace(p.getenv(name)) != "" {
			return true
		}
	}
	return false
}

func (p *parser) singleServer() Game {
	g := p.serverWithPrefix("default", "SDTD_")
	if g.Name == "default" && g.Host != "" {
		// Without an explicit name, the host is more use than "default".
		g.Name = g.Host
	}
	return g
}

// serverWithPrefix reads one server's settings from variables sharing a prefix.
func (p *parser) serverWithPrefix(id, prefix string) Game {
	g := Game{ID: id}
	g.Name = p.str(prefix+"NAME", id)
	g.Host = p.required(prefix + "HOST")
	g.Port = p.port(prefix+"API_PORT", 8080)
	g.Scheme = p.enum(prefix+"API_SCHEME", "http", "http", "https")
	g.TokenName = p.required(prefix + "API_TOKEN_NAME")
	g.TokenSecret = p.required(prefix + "API_TOKEN_SECRET")
	return g
}
