package console

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Everything in this file exists because the game console takes a single
// space-separated string, and much of what goes into it is attacker
// influenced: a player can name themselves almost anything, and item and buff
// names come from catalogues the server controls.
//
// Three rules, in order of preference:
//
//  1. Prefer integers. Every command that accepts an entity id is called with
//     the entity id rather than the name. An int cannot carry a payload.
//  2. Validate identifiers against a strict character set, and against the
//     server's own catalogue at the call site.
//  3. Reject free text rather than escaping it. Guessing the game's quoting
//     rules is a worse bet than refusing a newline.

// identifier matches the item, buff and entity class names the game uses.
// Every name observed in /api/item and /api/entityclass fits this.
var identifier = regexp.MustCompile(`^[A-Za-z0-9_.\-]+$`)

// userID matches a platform user id such as Steam_76561198021925107, the form
// ban accepts for offline players.
var userID = regexp.MustCompile(`^[A-Za-z]+_[A-Za-z0-9]+$`)

// maxFreeText bounds a ban reason or chat message.
const maxFreeText = 200

// ErrUnsafe is returned when an argument cannot be safely placed in a command.
type ErrUnsafe struct {
	Field  string
	Reason string
}

func (e *ErrUnsafe) Error() string { return e.Field + " " + e.Reason }

func checkIdentifier(field, value string) error {
	if value == "" {
		return &ErrUnsafe{field, "is required"}
	}
	if len(value) > 64 {
		return &ErrUnsafe{field, "is too long"}
	}
	if !identifier.MatchString(value) {
		return &ErrUnsafe{field, "may only contain letters, digits, dot, dash and underscore"}
	}
	return nil
}

// checkFreeText validates text that a person wrote, such as a ban reason.
//
// Quotes are rejected rather than escaped: the game's own quoting rules are
// undocumented, and a wrong guess turns a reason into extra arguments.
func checkFreeText(field, value string) error {
	if len(value) > maxFreeText {
		return &ErrUnsafe{field, fmt.Sprintf("is longer than %d characters", maxFreeText)}
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return &ErrUnsafe{field, "must not contain line breaks"}
	}
	if strings.ContainsAny(value, `"'`) {
		return &ErrUnsafe{field, "must not contain quotes"}
	}
	return nil
}

func checkEntityID(id int) error {
	if id < 0 {
		return &ErrUnsafe{"entity id", "must not be negative"}
	}
	return nil
}

// Teleport moves a player to a position.
//
// y = -1 asks the game to drop the player onto the ground, which is almost
// always what an operator wants rather than a precise height.
func Teleport(entityID, x, y, z int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	return fmt.Sprintf("teleportplayer %d %d %d %d", entityID, x, y, z), nil
}

// TeleportToPlayer moves one player to another.
func TeleportToPlayer(entityID, targetEntityID int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if err := checkEntityID(targetEntityID); err != nil {
		return "", err
	}
	if entityID == targetEntityID {
		return "", &ErrUnsafe{"target", "is the same player"}
	}
	return fmt.Sprintf("teleportplayer %d %d", entityID, targetEntityID), nil
}

// GiveItem drops an item in front of a player. Quality below 1 is omitted, for
// items that have no quality.
func GiveItem(entityID int, item string, count, quality int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if err := checkIdentifier("item name", item); err != nil {
		return "", err
	}
	if count < 1 {
		return "", &ErrUnsafe{"count", "must be at least 1"}
	}
	if count > 10000 {
		return "", &ErrUnsafe{"count", "is unreasonably large"}
	}
	if quality > 0 {
		if quality > 6 {
			return "", &ErrUnsafe{"quality", "must be between 1 and 6"}
		}
		return fmt.Sprintf("give %d %s %d %d", entityID, item, count, quality), nil
	}
	return fmt.Sprintf("give %d %s %d", entityID, item, count), nil
}

// Kill kills a player or entity.
func Kill(entityID int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	return fmt.Sprintf("kill %d", entityID), nil
}

// Kick disconnects a player, with an optional reason.
func Kick(entityID int, reason string) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if err := checkFreeText("reason", reason); err != nil {
		return "", err
	}
	if reason == "" {
		return fmt.Sprintf("kick %d", entityID), nil
	}
	return fmt.Sprintf("kick %d %s", entityID, reason), nil
}

// BanUnit is a duration unit the ban command accepts.
type BanUnit string

const (
	BanMinutes BanUnit = "minutes"
	BanHours   BanUnit = "hours"
	BanDays    BanUnit = "days"
	BanWeeks   BanUnit = "weeks"
	BanMonths  BanUnit = "months"
	BanYears   BanUnit = "years"
)

var banUnits = map[BanUnit]bool{
	BanMinutes: true, BanHours: true, BanDays: true,
	BanWeeks: true, BanMonths: true, BanYears: true,
}

// Ban bans a player by platform user id.
//
// The id form is used rather than the name because it is the only variant that
// works for players who are currently offline, and because an id cannot carry
// a payload the way a chosen display name can.
func Ban(platformUserID string, duration int, unit BanUnit, reason string) (string, error) {
	if !userID.MatchString(platformUserID) {
		return "", &ErrUnsafe{"user id", "must look like Steam_7656119..."}
	}
	if duration < 1 {
		return "", &ErrUnsafe{"duration", "must be at least 1"}
	}
	if !banUnits[unit] {
		return "", &ErrUnsafe{"duration unit", "is not one the server accepts"}
	}
	if err := checkFreeText("reason", reason); err != nil {
		return "", err
	}
	base := fmt.Sprintf("ban add %s %d %s", platformUserID, duration, unit)
	if reason == "" {
		return base, nil
	}
	return base + " " + reason, nil
}

// Unban lifts a ban.
func Unban(platformUserID string) (string, error) {
	if !userID.MatchString(platformUserID) {
		return "", &ErrUnsafe{"user id", "must look like Steam_7656119..."}
	}
	return "ban remove " + platformUserID, nil
}

// Buff applies a buff to a player. Buff names come from the game's own list.
func Buff(entityID int, buff string) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if err := checkIdentifier("buff name", buff); err != nil {
		return "", err
	}
	return fmt.Sprintf("buffplayer %d %s", entityID, buff), nil
}

// Debuff removes a buff from a player.
func Debuff(entityID int, buff string) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if err := checkIdentifier("buff name", buff); err != nil {
		return "", err
	}
	return fmt.Sprintf("debuffplayer %d %s", entityID, buff), nil
}

// GiveXP grants experience.
//
// There is no command to set a player's level: the game exposes only additive
// XP, so a level cannot be lowered and cannot be set precisely.
func GiveXP(entityID, amount int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	if amount < 1 {
		return "", &ErrUnsafe{"amount", "must be at least 1"}
	}
	if amount > 10_000_000 {
		return "", &ErrUnsafe{"amount", "is unreasonably large"}
	}
	return fmt.Sprintf("givexp %d %d", entityID, amount), nil
}

// SetTime sets the in-game clock.
func SetTime(day, hour, minute int) (string, error) {
	if day < 1 {
		return "", &ErrUnsafe{"day", "must be at least 1"}
	}
	if hour < 0 || hour > 23 {
		return "", &ErrUnsafe{"hour", "must be between 0 and 23"}
	}
	if minute < 0 || minute > 59 {
		return "", &ErrUnsafe{"minute", "must be between 0 and 59"}
	}
	return fmt.Sprintf("settime %d %d %d", day, hour, minute), nil
}

// WeatherKnob is a weather parameter the weather command accepts.
type WeatherKnob string

const (
	WeatherClouds   WeatherKnob = "Clouds"
	WeatherRain     WeatherKnob = "Rain"
	WeatherSnowFall WeatherKnob = "SnowFall"
	WeatherWind     WeatherKnob = "Wind"
	WeatherTemp     WeatherKnob = "Temp"
	WeatherFog      WeatherKnob = "Fog"
)

// weatherRanges are the bounds the game's own help text documents.
var weatherRanges = map[WeatherKnob][2]float64{
	WeatherClouds:   {0, 1},
	WeatherRain:     {0, 1},
	WeatherSnowFall: {0, 1},
	WeatherFog:      {0, 1},
	WeatherWind:     {0, 200},
	WeatherTemp:     {-99, 101},
}

// Weather sets one weather parameter.
func Weather(knob WeatherKnob, value float64) (string, error) {
	bounds, ok := weatherRanges[knob]
	if !ok {
		return "", &ErrUnsafe{"weather setting", "is not one the server accepts"}
	}
	if value < bounds[0] || value > bounds[1] {
		return "", &ErrUnsafe{string(knob),
			fmt.Sprintf("must be between %g and %g", bounds[0], bounds[1])}
	}
	return fmt.Sprintf("weather %s %s", knob,
		strconv.FormatFloat(value, 'g', -1, 64)), nil
}

// WeatherDefaults returns the weather to its simulated behaviour.
func WeatherDefaults() string { return "weather Defaults" }

// SpawnEntityAt spawns entities at a position.
//
// The command takes the entity class *name*, not the id from
// /api/entityclass. That id is a hash such as -1440238285, and passing it gets
// "Entity class name '-1440238285' unknown or not allowed to be instantiated".
// The name is safe to interpolate because it is validated against the
// identifier charset and checked against the server's own catalogue first.
func SpawnEntityAt(entityClass string, x, y, z, count int) (string, error) {
	if err := checkIdentifier("entity class", entityClass); err != nil {
		return "", err
	}
	if count < 1 {
		return "", &ErrUnsafe{"count", "must be at least 1"}
	}
	if count > 100 {
		return "", &ErrUnsafe{"count", "must be 100 or fewer"}
	}
	return fmt.Sprintf("spawnentityat %s %d %d %d %d",
		entityClass, x, y, z, count), nil
}

// SpawnWanderingHorde starts a wandering horde.
//
// This is not a blood moon. /api/bloodmoon is read-only and there is no
// bloodmoon command, so the only way to actually trigger one is to move the
// clock to its day with SetTime.
func SpawnWanderingHorde() string { return "spawnwandering" }

// SpawnScouts spawns screamer scouts at a player.
func SpawnScouts(entityID int) (string, error) {
	if err := checkEntityID(entityID); err != nil {
		return "", err
	}
	return fmt.Sprintf("spawnscouts %d", entityID), nil
}

// Say broadcasts a message to every player.
func Say(message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", &ErrUnsafe{"message", "is required"}
	}
	if err := checkFreeText("message", message); err != nil {
		return "", err
	}
	return "say " + message, nil
}

// SetGamePref changes a game preference at runtime.
//
// Runtime only: the game writes nothing back to serverconfig.xml, so the value
// reverts on restart. Callers must say so.
func SetGamePref(name, value string) (string, error) {
	if err := checkIdentifier("setting name", name); err != nil {
		return "", err
	}
	if err := checkFreeText("value", value); err != nil {
		return "", err
	}
	if strings.ContainsAny(value, " \t") {
		return "", &ErrUnsafe{"value", "must not contain spaces"}
	}
	if value == "" {
		return "", &ErrUnsafe{"value", "is required"}
	}
	return fmt.Sprintf("setgamepref %s %s", name, value), nil
}
