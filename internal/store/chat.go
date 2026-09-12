package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Audience is who may run a chat command.
type Audience string

const (
	// AudienceEveryone lets any connected player run it.
	AudienceEveryone Audience = "everyone"
	// AudienceAdmins restricts it to the game's own admin list, which is the
	// identity the people typing in chat actually have.
	AudienceAdmins Audience = "admins"
)

// ChatCommand is one command's configuration.
type ChatCommand struct {
	Name            string    `json:"name"`
	Enabled         bool      `json:"enabled"`
	Audience        Audience  `json:"audience"`
	CooldownSeconds int       `json:"cooldownSeconds"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// Kit is a named basket of items.
type Kit struct {
	Name string `json:"name"`
	// Items is the basket as stored: a JSON array of {item, count, quality}.
	Items     string    `json:"items"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ChatCommands lists one server's configured commands, in name order.
func (s *Store) ChatCommands(ctx context.Context, serverID string) ([]ChatCommand, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, enabled, audience, cooldown_seconds, updated_at
		   FROM chat_commands WHERE server_id = ? ORDER BY name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("store: list chat commands: %w", err)
	}
	defer rows.Close()

	var out []ChatCommand
	for rows.Next() {
		var c ChatCommand
		var enabled int
		var updated int64
		if err := rows.Scan(&c.Name, &enabled, &c.Audience, &c.CooldownSeconds, &updated); err != nil {
			return nil, fmt.Errorf("store: scan chat command: %w", err)
		}
		c.Enabled = enabled == 1
		c.UpdatedAt = time.Unix(updated, 0).UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveChatCommand writes one command's configuration.
func (s *Store) SaveChatCommand(ctx context.Context, serverID string, c ChatCommand, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_commands (server_id, name, enabled, audience, cooldown_seconds, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (server_id, name) DO UPDATE SET
		   enabled = excluded.enabled,
		   audience = excluded.audience,
		   cooldown_seconds = excluded.cooldown_seconds,
		   updated_at = excluded.updated_at`,
		serverID, c.Name, boolToInt(c.Enabled), string(c.Audience), c.CooldownSeconds, now.Unix())
	if err != nil {
		return fmt.Errorf("store: save chat command: %w", err)
	}
	return nil
}

/*
TakeCooldown records a use if the cooldown has expired, and reports whether it
did.

One statement rather than a read followed by a write: two players running the
same command in the same instant would both read "not used recently" and both
be allowed through. The insert only lands when the stored time is old enough,
so the database decides rather than the caller.

Returns how long is left when it refuses, because "wait" is a worse answer than
"wait four minutes".
*/
func (s *Store) TakeCooldown(
	ctx context.Context, serverID, platformID, command string, cooldown time.Duration, now time.Time,
) (bool, time.Duration, error) {
	if cooldown <= 0 {
		return true, 0, nil
	}
	cutoff := now.Add(-cooldown).Unix()

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_cooldowns (server_id, platform_id, command, used_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (server_id, platform_id, command) DO UPDATE SET used_at = excluded.used_at
		   WHERE chat_cooldowns.used_at <= ?`,
		serverID, platformID, command, now.Unix(), cutoff)
	if err != nil {
		return false, 0, fmt.Errorf("store: take cooldown: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return false, 0, fmt.Errorf("store: take cooldown: %w", err)
	}
	if changed > 0 {
		return true, 0, nil
	}

	var usedAt int64
	err = s.db.QueryRowContext(ctx,
		`SELECT used_at FROM chat_cooldowns
		   WHERE server_id = ? AND platform_id = ? AND command = ?`,
		serverID, platformID, command).Scan(&usedAt)
	if err != nil {
		return false, 0, fmt.Errorf("store: read cooldown: %w", err)
	}
	left := time.Unix(usedAt, 0).Add(cooldown).Sub(now)
	if left < 0 {
		left = 0
	}
	return false, left, nil
}

/*
ClearCooldown forgets a use, so a command that failed does not cost a player an
hour of waiting for nothing.

Deleting rather than back-dating: the row only exists to answer "how long ago",
and a row that is not there means the same as one that is old enough.
*/
func (s *Store) ClearCooldown(ctx context.Context, serverID, platformID, command string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM chat_cooldowns
		   WHERE server_id = ? AND platform_id = ? AND command = ?`,
		serverID, platformID, command)
	if err != nil {
		return fmt.Errorf("store: clear cooldown: %w", err)
	}
	return nil
}

// Kits lists every kit, in name order.
func (s *Store) Kits(ctx context.Context) ([]Kit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, items, created_at, updated_at FROM kits ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list kits: %w", err)
	}
	defer rows.Close()

	var out []Kit
	for rows.Next() {
		var k Kit
		var created, updated int64
		if err := rows.Scan(&k.Name, &k.Items, &created, &updated); err != nil {
			return nil, fmt.Errorf("store: scan kit: %w", err)
		}
		k.CreatedAt = time.Unix(created, 0).UTC()
		k.UpdatedAt = time.Unix(updated, 0).UTC()
		out = append(out, k)
	}
	return out, rows.Err()
}

// Kit reads one kit by name.
func (s *Store) Kit(ctx context.Context, name string) (Kit, error) {
	var k Kit
	var created, updated int64
	err := s.db.QueryRowContext(ctx,
		`SELECT name, items, created_at, updated_at FROM kits WHERE name = ?`, name).
		Scan(&k.Name, &k.Items, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Kit{}, ErrNotFound
	}
	if err != nil {
		return Kit{}, fmt.Errorf("store: read kit: %w", err)
	}
	k.CreatedAt = time.Unix(created, 0).UTC()
	k.UpdatedAt = time.Unix(updated, 0).UTC()
	return k, nil
}

// SaveKit creates or replaces a kit.
func (s *Store) SaveKit(ctx context.Context, name, items string, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kits (name, items, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (name) DO UPDATE SET items = excluded.items, updated_at = excluded.updated_at`,
		name, items, now.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("store: save kit: %w", err)
	}
	return nil
}

// DeleteKit removes a kit.
func (s *Store) DeleteKit(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM kits WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("store: delete kit: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
