package sdtd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Player is one player, merged from the two endpoints that know about them.
//
// Neither is sufficient alone. /api/player covers only players who are online
// right now, and hard-codes totalPlayTimeSeconds and lastOnline to null. The
// undocumented /api/getplayerlist knows about everyone who has ever joined, and
// carries playtime and last-seen, but not health, deaths or kills.
type Player struct {
	EntityID        int      `json:"entityId"`
	Name            string   `json:"name"`
	PlatformID      string   `json:"platformId"`
	CrossplatformID string   `json:"crossplatformId"`
	Online          bool     `json:"online"`
	IP              string   `json:"ip"`
	Ping            int      `json:"ping"`
	Position        Position `json:"position"`

	// Level comes from /api/player and so is only known while online. The spec
	// declares it permanently null; a live server sends an integer.
	Level       int     `json:"level"`
	Health      int     `json:"health"`
	Stamina     float64 `json:"stamina"`
	Score       int     `json:"score"`
	Deaths      int     `json:"deaths"`
	ZombieKills int     `json:"zombieKills"`
	PlayerKills int     `json:"playerKills"`

	// PlayTimeSeconds and LastOnline come from the legacy endpoint and are
	// known for offline players too.
	//
	// The upstream field is called totalplaytime, but it is not a lifetime
	// total on this build: it was observed resetting from 696s to 45s when a
	// player died, with no server restart. Treat it as time since the current
	// life began, and label it accordingly.
	PlayTimeSeconds int        `json:"playTimeSeconds"`
	LastOnline      *time.Time `json:"lastOnline"`

	Banned    bool       `json:"banned"`
	BanReason string     `json:"banReason,omitempty"`
	BanUntil  *time.Time `json:"banUntil,omitempty"`
}

// Position is a world position. The spec calls this a Vector3i and types the
// axes as integers, but a live server sends floats such as -272.96875.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// onlinePlayer mirrors /api/player.
type onlinePlayer struct {
	EntityID int    `json:"entityId"`
	Name     string `json:"name"`
	Platform *struct {
		Combined string `json:"combinedString"`
	} `json:"platformId"`
	Crossplatform *struct {
		Combined string `json:"combinedString"`
	} `json:"crossplatformId"`
	IP       *string   `json:"ip"`
	Ping     *int      `json:"ping"`
	Position *Position `json:"position"`
	Level    int       `json:"level"`
	Health   int       `json:"health"`
	Stamina  float64   `json:"stamina"`
	Score    int       `json:"score"`
	Deaths   int       `json:"deaths"`
	Kills    struct {
		Zombies int `json:"zombies"`
		Players int `json:"players"`
	} `json:"kills"`
	Banned struct {
		Active bool       `json:"banActive"`
		Reason *string    `json:"reason"`
		Until  *time.Time `json:"until"`
	} `json:"banned"`
}

type onlinePlayers struct {
	Players []onlinePlayer `json:"players"`
}

// knownPlayer mirrors one row of /api/getplayerlist.
//
// That endpoint is absent from the OpenAPI spec; its field names were read off
// the server's own legacymap client and confirmed against a live server.
type knownPlayer struct {
	SteamID         string `json:"steamid"`
	CrossplatformID string `json:"crossplatformid"`
	EntityID        int    `json:"entityid"`
	IP              string `json:"ip"`
	Name            string `json:"name"`
	Online          bool   `json:"online"`
	Position        struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"position"`
	TotalPlayTime int     `json:"totalplaytime"`
	LastOnline    *string `json:"lastonline"`
	Ping          int     `json:"ping"`
	Banned        bool    `json:"banned"`
}

type knownPlayers struct {
	Total    int           `json:"total"`
	Players  []knownPlayer `json:"players"`
	FirstRow int           `json:"firstResult"`
}

// Players returns every player the server knows about, online first.
//
// The two sources are merged on the platform user id rather than the entity
// id: entity ids are reassigned between sessions, so the same person has a
// different one each time they join.
func (c *Client) Players(ctx context.Context) ([]Player, error) {
	known, knownErr := c.knownPlayers(ctx)
	online, onlineErr := c.onlinePlayers(ctx)

	// The legacy endpoint is the one that is not in the spec, so if it has gone
	// away the panel still shows who is online rather than nothing at all.
	if knownErr != nil && onlineErr != nil {
		return nil, knownErr
	}

	byID := make(map[string]*Player, len(known)+len(online))
	var order []string

	for _, k := range known {
		id := k.SteamID
		if id == "" {
			id = k.CrossplatformID
		}
		p := &Player{
			EntityID:        k.EntityID,
			Name:            k.Name,
			PlatformID:      k.SteamID,
			CrossplatformID: k.CrossplatformID,
			Online:          k.Online,
			IP:              k.IP,
			Ping:            k.Ping,
			Position:        Position{X: k.Position.X, Y: k.Position.Y, Z: k.Position.Z},
			PlayTimeSeconds: k.TotalPlayTime,
			Banned:          k.Banned,
		}
		if k.LastOnline != nil {
			if t, err := time.Parse(time.RFC3339, *k.LastOnline); err == nil {
				p.LastOnline = &t
			}
		}
		byID[id] = p
		order = append(order, id)
	}

	for _, o := range online {
		id := ""
		if o.Platform != nil {
			id = o.Platform.Combined
		}
		if id == "" && o.Crossplatform != nil {
			id = o.Crossplatform.Combined
		}

		p, ok := byID[id]
		if !ok {
			p = &Player{}
			byID[id] = p
			order = append(order, id)
		}

		// The online endpoint is authoritative for anyone currently connected.
		p.EntityID = o.EntityID
		p.Name = o.Name
		p.Online = true
		p.Level = o.Level
		p.Health = o.Health
		p.Stamina = o.Stamina
		p.Score = o.Score
		p.Deaths = o.Deaths
		p.ZombieKills = o.Kills.Zombies
		p.PlayerKills = o.Kills.Players
		p.Banned = p.Banned || o.Banned.Active
		if o.Banned.Reason != nil {
			p.BanReason = *o.Banned.Reason
		}
		p.BanUntil = o.Banned.Until
		if o.Platform != nil {
			p.PlatformID = o.Platform.Combined
		}
		if o.Crossplatform != nil {
			p.CrossplatformID = o.Crossplatform.Combined
		}
		if o.IP != nil {
			p.IP = *o.IP
		}
		if o.Ping != nil {
			p.Ping = *o.Ping
		}
		if o.Position != nil {
			p.Position = *o.Position
		}
	}

	out := make([]Player, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func (c *Client) onlinePlayers(ctx context.Context) ([]onlinePlayer, error) {
	result, err := fetch[onlinePlayers](func() (*http.Response, error) {
		return c.gen.PlayerGet(ctx)
	})
	if err != nil {
		return nil, err
	}
	return result.Players, nil
}

// knownPlayers calls /api/getplayerlist, which is not in the OpenAPI spec and
// so is not covered by the generated client.
func (c *Client) knownPlayers(ctx context.Context) ([]knownPlayer, error) {
	params := url.Values{}
	// page is zero-based: the server returns firstResult = page * rowsperpage,
	// so page=1 skips the first rowsperpage rows and a small server returns
	// nothing at all.
	params.Set("page", "0")
	params.Set("rowsperpage", strconv.Itoa(maxKnownPlayers))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/getplayerlist?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderTokenName, c.tokenName)
	req.Header.Set(HeaderTokenSecret, c.tokenSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.gen.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sdtd: get player list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: resp.StatusCode, RequestSubpath: "getplayerlist"}
	}

	// This endpoint returns a bare object, not the {data, meta} envelope the
	// documented API uses.
	var page knownPlayers
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("sdtd: decode player list: %w", err)
	}
	return page.Players, nil
}

// maxKnownPlayers bounds how many rows are requested in one go.
const maxKnownPlayers = 500
