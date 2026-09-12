package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

// Kind separates the commands the panel ships with from the ones an admin
// wrote.
type Kind string

const (
	// KindBuiltin keeps its behaviour in internal/chat and uses only the
	// enabled, audience and cooldown columns.
	KindBuiltin Kind = "builtin"
	// KindCustom carries its whole behaviour in the row.
	KindCustom Kind = "custom"
)

// ChatCommand is one command's configuration, and for a custom command its
// behaviour too.
type ChatCommand struct {
	Name            string   `json:"name"`
	Kind            Kind     `json:"kind"`
	Enabled         bool     `json:"enabled"`
	Audience        Audience `json:"audience"`
	CooldownSeconds int      `json:"cooldownSeconds"`

	// Description is what the command does, in the admin's words. Built-ins
	// have their own and leave this empty.
	Description string `json:"description,omitempty"`
	// Reply is what the player is told. A command with a reply and no commands
	// is one that only answers: !discord, !rules.
	Reply string `json:"reply,omitempty"`
	// Commands are console lines to run, in order. The game's console takes one
	// at a time, so a list here is what lets one chat command be a sequence.
	Commands []string `json:"commands,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
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
		`SELECT name, kind, enabled, audience, cooldown_seconds,
		        description, reply, commands, updated_at
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
		var commands string
		if err := rows.Scan(&c.Name, &c.Kind, &enabled, &c.Audience, &c.CooldownSeconds,
			&c.Description, &c.Reply, &commands, &updated); err != nil {
			return nil, fmt.Errorf("store: scan chat command: %w", err)
		}
		c.Enabled = enabled == 1
		c.UpdatedAt = time.Unix(updated, 0).UTC()
		// A row whose command list will not parse is still worth returning: an
		// admin can see it and repair it, where a row that vanished from the
		// list would just look like the panel had lost it.
		if err := json.Unmarshal([]byte(commands), &c.Commands); err != nil {
			c.Commands = nil
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveChatCommand writes one command's configuration, and its behaviour when
// it has one.
func (s *Store) SaveChatCommand(ctx context.Context, serverID string, c ChatCommand, now time.Time) error {
	if c.Kind == "" {
		c.Kind = KindBuiltin
	}
	commands := c.Commands
	if commands == nil {
		commands = []string{}
	}
	encoded, err := json.Marshal(commands)
	if err != nil {
		return fmt.Errorf("store: encode chat command lines: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO chat_commands
		   (server_id, name, kind, enabled, audience, cooldown_seconds,
		    description, reply, commands, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (server_id, name) DO UPDATE SET
		   kind = excluded.kind,
		   enabled = excluded.enabled,
		   audience = excluded.audience,
		   cooldown_seconds = excluded.cooldown_seconds,
		   description = excluded.description,
		   reply = excluded.reply,
		   commands = excluded.commands,
		   updated_at = excluded.updated_at`,
		serverID, c.Name, string(c.Kind), boolToInt(c.Enabled), string(c.Audience),
		c.CooldownSeconds, c.Description, c.Reply, string(encoded), now.Unix())
	if err != nil {
		return fmt.Errorf("store: save chat command: %w", err)
	}
	return nil
}

// DeleteChatCommand removes a command's row.
//
// For a custom command this is the command ceasing to exist. For a built-in it
// is only the configuration going away, which leaves it switched off, since a
// built-in with no row is one nobody has turned on.
func (s *Store) DeleteChatCommand(ctx context.Context, serverID, name string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM chat_commands WHERE server_id = ? AND name = ?`, serverID, name)
	if err != nil {
		return fmt.Errorf("store: delete chat command: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	// The cooldowns it accumulated are meaningless once it is gone, and would
	// otherwise be waiting for anybody who recreated the same name.
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM chat_cooldowns WHERE server_id = ? AND command = ?`, serverID, name); err != nil {
		return fmt.Errorf("store: delete cooldowns for %s: %w", name, err)
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
