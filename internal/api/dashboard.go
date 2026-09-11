package api

import (
	"net/http"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

type dashboardResponse struct {
	// Status is one of unknown, online, degraded or offline.
	Status string `json:"status"`
	// Stale is true when the figures below are cached from an earlier
	// successful poll rather than fresh. The UI shows their age instead of
	// blanking, so a blip does not make the dashboard look empty.
	Stale bool `json:"stale"`
	// AgeSeconds is how old the cached stats are.
	AgeSeconds float64 `json:"ageSeconds"`
	// LastError is the most recent poll failure, empty while healthy.
	LastError string `json:"lastError,omitempty"`

	Server  dashboardServer     `json:"server"`
	Players dashboardPlayers    `json:"players"`
	World   dashboardWorld      `json:"world"`
	Uptime  *dashboardUptime    `json:"uptime"`
	Moon    *dashboardBloodmoon `json:"bloodMoon"`
}

type dashboardServer struct {
	// Version is the game server's own ServerVersion string. It is not
	// identical to what the console version command reports for the same
	// build; reading it here avoids executing a command, which would write a
	// line into the log feed on every poll.
	Version  string `json:"version"`
	GameMode string `json:"gameMode"`
}

type dashboardPlayers struct {
	Online int `json:"online"`
	Max    int `json:"max"`
}

type dashboardWorld struct {
	Name     string `json:"name"`
	Day      int    `json:"day"`
	Hour     int    `json:"hour"`
	Minute   int    `json:"minute"`
	Hostiles int    `json:"hostiles"`
	Animals  int    `json:"animals"`
}

type dashboardUptime struct {
	Seconds float64 `json:"seconds"`
}

type dashboardBloodmoon struct {
	Active  bool `json:"active"`
	NextDay int  `json:"nextDay"`
	// NextHour is the in-game hour the blood moon begins.
	NextHour int `json:"nextHour"`
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	snap := s.state.Snapshot()
	now := s.now()

	resp := dashboardResponse{
		Status:    string(snap.Status),
		LastError: snap.LastError,
		Server: dashboardServer{
			Version:  snap.Version,
			GameMode: snap.GameMode,
		},
		Players: dashboardPlayers{
			Online: snap.Stats.Players,
			Max:    snap.MaxPlayers,
		},
		World: dashboardWorld{
			Name:     snap.World,
			Day:      snap.Stats.GameTime.Days,
			Hour:     snap.Stats.GameTime.Hours,
			Minute:   snap.Stats.GameTime.Minutes,
			Hostiles: snap.Stats.Hostiles,
			Animals:  snap.Stats.Animals,
		},
	}

	if age, ok := snap.StatsAge(now); ok {
		resp.AgeSeconds = age.Seconds()
		// Anything older than a few poll intervals is worth flagging even if
		// the status has not yet degraded.
		resp.Stale = snap.Status != state.StatusOnline
	} else {
		resp.Stale = true
	}

	if d, ok := snap.UptimeAt(now); ok {
		resp.Uptime = &dashboardUptime{Seconds: d.Round(time.Second).Seconds()}
	}
	if !snap.BloodmoonAt.IsZero() {
		resp.Moon = &dashboardBloodmoon{
			Active:   snap.Bloodmoon.Active,
			NextDay:  snap.Bloodmoon.Next.Days,
			NextHour: snap.Bloodmoon.Next.Hours,
		}
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}
