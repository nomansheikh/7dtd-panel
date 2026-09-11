// Package sdtd talks to a 7 Days to Die dedicated server's web API.
//
// It wraps the generated client in internal/sdtd/gen, which supplies correct
// paths and query encoding but returns raw *http.Response. This package owns
// everything above that: the API token headers, the {data, meta} envelope, and
// turning meta.errorCode into a typed APIError.
//
// Nothing here is aware of HTTP requests from a browser. The browser never
// talks to the game server; the panel proxies every call.
package sdtd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd/gen"
)

// Header names the game server uses for token authentication. Both are
// required together.
const (
	HeaderTokenName   = "X-SDTD-API-TOKENNAME"
	HeaderTokenSecret = "X-SDTD-API-SECRET"
)

// maxBodyBytes caps how much we will read from any single response.
// /api/item is ~2.9 MB on a stock install, so the ceiling has to be generous
// while still bounding a hostile or broken server.
const maxBodyBytes = 64 << 20

// Client is a 7 Days to Die web API client. It is safe for concurrent use.
type Client struct {
	gen     *gen.Client
	baseURL string

	// Retained for the log stream, which is not in the OpenAPI spec and so is
	// not covered by the generated client.
	tokenName   string
	tokenSecret string
	// streamHTTP has no overall timeout, because an SSE response body stays
	// open indefinitely and the regular client's timeout would sever it. Header
	// and dial timeouts still bound how long a dead server can hang a connect.
	streamHTTP *http.Client
}

// Options configures a Client.
type Options struct {
	BaseURL     string
	TokenName   string
	TokenSecret string
	// Timeout applies per request. Zero selects a sane default.
	Timeout time.Duration
	// HTTPClient overrides the transport, for tests.
	HTTPClient *http.Client
	// StreamHTTPClient overrides the transport used for the log stream. It must
	// not set an overall Timeout.
	StreamHTTPClient *http.Client
}

// New builds a Client. It does not contact the server; a Client is usable even
// when the game server is down, which is what lets the panel start and serve a
// clear disconnected state rather than failing to boot.
func New(opts Options) (*Client, error) {
	if opts.BaseURL == "" {
		return nil, errors.New("sdtd: BaseURL is required")
	}
	if opts.TokenName == "" || opts.TokenSecret == "" {
		return nil, errors.New("sdtd: TokenName and TokenSecret are required")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	auth := func(_ context.Context, req *http.Request) error {
		req.Header.Set(HeaderTokenName, opts.TokenName)
		req.Header.Set(HeaderTokenSecret, opts.TokenSecret)
		req.Header.Set("Accept", "application/json")
		return nil
	}

	g, err := gen.NewClient(opts.BaseURL,
		gen.WithHTTPClient(httpClient),
		gen.WithRequestEditorFn(auth),
	)
	if err != nil {
		return nil, fmt.Errorf("sdtd: build client: %w", err)
	}

	streamHTTP := opts.StreamHTTPClient
	if streamHTTP == nil {
		streamHTTP = &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				ResponseHeaderTimeout: 15 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		}
	}

	return &Client{
		gen:         g,
		baseURL:     opts.BaseURL,
		tokenName:   opts.TokenName,
		tokenSecret: opts.TokenSecret,
		streamHTTP:  streamHTTP,
	}, nil
}

// BaseURL reports the configured game server root.
func (c *Client) BaseURL() string { return c.baseURL }

// envelope is the wrapper every JSON endpoint returns.
type envelope[T any] struct {
	Data T    `json:"data"`
	Meta meta `json:"meta"`
}

type meta struct {
	ServerTime       string `json:"serverTime"`
	RequestMethod    string `json:"requestMethod"`
	RequestSubpath   string `json:"requestSubpath"`
	ErrorCode        string `json:"errorCode"`
	ExceptionMessage string `json:"exceptionMessage"`
	ExceptionTrace   string `json:"exceptionTrace"`
}

// fetch performs call and decodes the envelope's data into T.
//
// A non-2xx status always produces an *APIError, populated from meta when the
// body is a readable envelope. The server answers errors with a JSON envelope
// in every case observed, but a bare status is handled too.
func fetch[T any](call func() (*http.Response, error)) (T, error) {
	var zero T

	resp, err := call()
	if err != nil {
		return zero, fmt.Errorf("sdtd: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return zero, fmt.Errorf("sdtd: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{Status: resp.StatusCode}
		var env envelope[json.RawMessage]
		if json.Unmarshal(body, &env) == nil {
			apiErr.ErrorCode = env.Meta.ErrorCode
			apiErr.RequestMethod = env.Meta.RequestMethod
			apiErr.RequestSubpath = env.Meta.RequestSubpath
			apiErr.ExceptionMessage = env.Meta.ExceptionMessage
			apiErr.Trace = env.Meta.ExceptionTrace
		}
		return zero, apiErr
	}

	var env envelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, fmt.Errorf("sdtd: decode response: %w", err)
	}
	// A 200 carrying an errorCode has been seen on endpoints that answer
	// generically, so treat it as a failure rather than returning zero data.
	if env.Meta.ErrorCode != "" {
		return zero, &APIError{
			Status:           resp.StatusCode,
			ErrorCode:        env.Meta.ErrorCode,
			RequestMethod:    env.Meta.RequestMethod,
			RequestSubpath:   env.Meta.RequestSubpath,
			ExceptionMessage: env.Meta.ExceptionMessage,
			Trace:            env.Meta.ExceptionTrace,
		}
	}
	return env.Data, nil
}

// ---------- domain types ----------

// GameTime is the in-game clock.
type GameTime struct {
	Days    int `json:"days"`
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

// ServerStats is the cheapest liveness probe the server offers (~150 bytes),
// which is why the poller uses it as the health signal.
type ServerStats struct {
	GameTime GameTime `json:"gameTime"`
	Players  int      `json:"players"`
	Hostiles int      `json:"hostiles"`
	Animals  int      `json:"animals"`
}

// Bloodmoon describes the blood moon schedule. Read-only: the endpoint has no
// write method, so triggering a horde night goes through a console command.
type Bloodmoon struct {
	GameTime         GameTime `json:"gameTime"`
	Active           bool     `json:"bloodmoonActive"`
	Next             GameTime `json:"nextBloodmoon"`
	NextBloodmoonEnd GameTime `json:"nextBloodmoonEnd"`
}

// LogEntry is one line of the server log. The same shape arrives over the SSE
// stream as a logLine event.
type LogEntry struct {
	ID      int    `json:"id"`
	Msg     string `json:"msg"`
	Type    string `json:"type"`
	Trace   string `json:"trace"`
	ISOTime string `json:"isotime"`
	// Uptime is milliseconds since the server started, delivered as a JSON
	// string rather than a number.
	Uptime string `json:"uptime"`
}

// UptimeDuration parses Uptime, reporting whether it was a valid number.
func (e LogEntry) UptimeDuration() (time.Duration, bool) {
	ms, err := strconv.ParseInt(e.Uptime, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

// Time parses ISOTime, reporting whether it was valid.
func (e LogEntry) Time() (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, e.ISOTime)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// LogPage is a window onto the server log. LastLine is the cursor to pass as
// firstLine next time to follow the log without gaps.
type LogPage struct {
	Entries   []LogEntry `json:"entries"`
	FirstLine int        `json:"firstLine"`
	LastLine  int        `json:"lastLine"`
}

// CommandResult is the outcome of a console command executed with format Full.
type CommandResult struct {
	Command    string `json:"command"`
	Parameters string `json:"parameters"`
	Result     string `json:"result"`
}

// ---------- calls ----------

// ServerStats fetches the current server stats.
func (c *Client) ServerStats(ctx context.Context) (ServerStats, error) {
	return fetch[ServerStats](func() (*http.Response, error) {
		return c.gen.ServerStatsGet(ctx)
	})
}

// ServerInfo fetches the server's static-ish info as a name-indexed set.
// ServerVersion, CurrentPlayers and MaxPlayers live here.
func (c *Client) ServerInfo(ctx context.Context) (ValueSet, error) {
	values, err := fetch[[]TypedValue](func() (*http.Response, error) {
		return c.gen.ServerInfoGet(ctx)
	})
	if err != nil {
		return ValueSet{}, err
	}
	return NewValueSet(values), nil
}

// GameStats fetches mutable game state as a name-indexed set.
func (c *Client) GameStats(ctx context.Context) (ValueSet, error) {
	values, err := fetch[[]TypedValue](func() (*http.Response, error) {
		return c.gen.GameStatsGet(ctx)
	})
	if err != nil {
		return ValueSet{}, err
	}
	return NewValueSet(values), nil
}

// GamePrefs fetches the server's preferences. Read-only over REST: PUT returns
// 405 Unsupported, so changing one means the setgamepref console command.
func (c *Client) GamePrefs(ctx context.Context) (ValueSet, error) {
	values, err := fetch[[]TypedValue](func() (*http.Response, error) {
		return c.gen.GamePrefsGet(ctx)
	})
	if err != nil {
		return ValueSet{}, err
	}
	return NewValueSet(values), nil
}

// Bloodmoon fetches the blood moon schedule.
func (c *Client) Bloodmoon(ctx context.Context) (Bloodmoon, error) {
	return fetch[Bloodmoon](func() (*http.Response, error) {
		return c.gen.BloodmoonGet(ctx)
	})
}

// Log fetches log lines. A positive count reads forward from firstLine; a
// negative count reads backward from the newest line. Pass firstLine < 0 to
// let the server choose.
func (c *Client) Log(ctx context.Context, firstLine, count int) (LogPage, error) {
	params := &gen.LogGetParams{}
	if count != 0 {
		params.Count = &count
	}
	if firstLine >= 0 {
		params.FirstLine = &firstLine
	}
	return fetch[LogPage](func() (*http.Response, error) {
		return c.gen.LogGet(ctx, params)
	})
}

// Execute runs a console command and returns its output.
//
// This is the only write path for game state: gameprefs, sandboxsettings and
// bloodmoon are all read-only over REST. Callers must not build command
// strings from unvalidated user input; use the builders in command.go.
func (c *Client) Execute(ctx context.Context, command string) (CommandResult, error) {
	full := gen.CommandPostJSONBodyFormatFull
	body := gen.CommandPostJSONRequestBody{Command: command, Format: &full}
	return fetch[CommandResult](func() (*http.Response, error) {
		return c.gen.CommandPost(ctx, body)
	})
}

// Command is one entry from the server's own command catalogue.
type Command struct {
	// Command is the primary name; Overloads lists every accepted alias,
	// including the primary one.
	Command     string   `json:"command"`
	Overloads   []string `json:"overloads"`
	Description string   `json:"description"`
	// Help is the server's multi-line usage text, absent for many commands.
	Help *string `json:"help"`
	// Allowed reports whether the current credentials may run it.
	Allowed *bool `json:"allowed"`
}

// commandsEnvelope matches the shape inside the response's data field.
type commandsEnvelope struct {
	Commands []Command `json:"commands"`
}

// Commands fetches the server's command catalogue.
//
// The panel builds its command palette from this rather than a hardcoded list,
// so the palette reflects what the server actually accepts, including commands
// added by mods, and carries the server's own help text.
func (c *Client) Commands(ctx context.Context) ([]Command, error) {
	env, err := fetch[commandsEnvelope](func() (*http.Response, error) {
		return c.gen.CommandGet(ctx)
	})
	if err != nil {
		return nil, err
	}
	return env.Commands, nil
}

// Item is one entry from the server's item catalogue.
type Item struct {
	Name          string `json:"name"`
	LocalizedName string `json:"localizedName"`
	IsBlock       bool   `json:"isBlock"`
}

// Items fetches the full item catalogue.
//
// This is roughly 2.9 MB on a stock install, which is why it is fetched once
// and indexed rather than proxied per request.
func (c *Client) Items(ctx context.Context) ([]Item, error) {
	return fetch[[]Item](func() (*http.Response, error) {
		return c.gen.ItemGet(ctx)
	})
}

// EntityClass is one spawnable entity type.
type EntityClass struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	// ManualSpawnType is "None" for things the game will not let you spawn.
	ManualSpawnType string `json:"manualSpawnType"`
}

// EntityClasses fetches the catalogue of entity types.
func (c *Client) EntityClasses(ctx context.Context) ([]EntityClass, error) {
	return fetch[[]EntityClass](func() (*http.Response, error) {
		return c.gen.EntityclassGet(ctx)
	})
}

// gamePrefLine matches the console's reply to getgamepref, which looks like
// "GamePref.AirDropFrequency = 72".
var gamePrefLine = regexp.MustCompile(`(?m)^\s*GamePref\.(\S+)\s*=\s*(.*?)\s*$`)

// GamePrefsLive returns every preference's live value via the console.
//
// /api/gameprefs reports the values the server started with, not the ones it is
// running: after setgamepref moved BloodMoonWarning to 2, that endpoint still
// said 1 while the console said 2. Anything that shows preferences has to
// overlay this, or it shows an operator a value the server is not using.
//
// The reply covers about 153 of the 287 preferences — the server-side ones.
// Preferences it omits keep whatever /api/gameprefs reported.
func (c *Client) GamePrefsLive(ctx context.Context) (map[string]string, error) {
	result, err := c.Execute(ctx, "getgamepref")
	if err != nil {
		return nil, err
	}
	matches := gamePrefLine.FindAllStringSubmatch(result.Result, -1)
	values := make(map[string]string, len(matches))
	for _, m := range matches {
		values[m[1]] = m[2]
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("sdtd: getgamepref returned nothing recognisable")
	}
	return values, nil
}

// ReadGamePref returns a preference's live value via the console.
//
// This exists because /api/gameprefs does not reflect runtime changes: after
// setgamepref moved AirDropFrequency to 72, that endpoint still reported 3
// while the console reported 72. Anything that writes a preference has to read
// it back this way or it will show the operator a stale value.
func (c *Client) ReadGamePref(ctx context.Context, name string) (string, error) {
	result, err := c.Execute(ctx, "getgamepref "+name)
	if err != nil {
		return "", err
	}
	for _, m := range gamePrefLine.FindAllStringSubmatch(result.Result, -1) {
		if strings.EqualFold(m[1], name) {
			return m[2], nil
		}
	}
	return "", fmt.Errorf("sdtd: no preference called %s", name)
}
