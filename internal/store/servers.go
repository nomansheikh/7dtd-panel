package store

import (
	"context"
	"fmt"
	"time"
)

// GameServer is one configured game server, as the panel stores it.
type GameServer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`

	TokenName string `json:"tokenName"`
	// TokenSecret is never sent to a browser. The panel proxies every game
	// call, so nothing in a tab has any use for it.
	TokenSecret string `json:"-"`

	Position  int       `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GameServers lists every configured server, in the order the UI shows them.
func (s *Store) GameServers(ctx context.Context) ([]GameServer, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, host, port, scheme, token_name, token_secret,
		        position, created_at, updated_at
		   FROM game_servers ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("store: list game servers: %w", err)
	}
	defer rows.Close()

	var out []GameServer
	for rows.Next() {
		var g GameServer
		var created, updated int64
		if err := rows.Scan(&g.ID, &g.Name, &g.Host, &g.Port, &g.Scheme,
			&g.TokenName, &g.TokenSecret, &g.Position, &created, &updated); err != nil {
			return nil, fmt.Errorf("store: scan game server: %w", err)
		}
		g.CreatedAt = time.Unix(created, 0).UTC()
		g.UpdatedAt = time.Unix(updated, 0).UTC()
		out = append(out, g)
	}
	return out, rows.Err()
}

// SaveGameServer creates or updates one.
//
// An empty TokenSecret on an existing row leaves the stored one alone, so an
// operator can rename a server without being made to paste its token again —
// and so the browser never has to be sent a secret in order to send it back.
func (s *Store) SaveGameServer(ctx context.Context, g GameServer, now time.Time) error {
	secret := g.TokenSecret
	if secret == "" {
		existing, err := s.GameServer(ctx, g.ID)
		if err != nil && err != ErrNotFound {
			return err
		}
		if err == nil {
			secret = existing.TokenSecret
		}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO game_servers
		   (id, name, host, port, scheme, token_name, token_secret, position,
		    created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   name = excluded.name,
		   host = excluded.host,
		   port = excluded.port,
		   scheme = excluded.scheme,
		   token_name = excluded.token_name,
		   token_secret = excluded.token_secret,
		   position = excluded.position,
		   updated_at = excluded.updated_at`,
		g.ID, g.Name, g.Host, g.Port, g.Scheme, g.TokenName, secret,
		g.Position, now.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("store: save game server: %w", err)
	}
	return nil
}

// GameServer reads one by id.
func (s *Store) GameServer(ctx context.Context, id string) (GameServer, error) {
	servers, err := s.GameServers(ctx)
	if err != nil {
		return GameServer{}, err
	}
	for _, g := range servers {
		if g.ID == id {
			return g, nil
		}
	}
	return GameServer{}, ErrNotFound
}

/*
DeleteGameServer removes a server and everything the panel kept about it.

Its chat commands, cooldowns and tasks go with it. They are meaningless
without it, and leaving them would mean a server added later under the same id
inheriting a stranger's automation — which is a worse surprise than losing
configuration that was already gone.
*/
func (s *Store) DeleteGameServer(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM game_servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete game server: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	for _, table := range []string{"chat_commands", "chat_cooldowns", "automation_tasks", "automation_runs"} {
		if _, err := s.db.ExecContext(ctx,
			`DELETE FROM `+table+` WHERE server_id = ?`, id); err != nil {
			return fmt.Errorf("store: clear %s for %s: %w", table, id, err)
		}
	}
	return nil
}
