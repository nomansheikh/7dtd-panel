package api

import (
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// commandRun builds a history record from an execution outcome, so the console
// and the world actions record themselves identically.
func commandRun(userID int64, command, result string, execErr error, now time.Time) store.CommandRun {
	run := store.CommandRun{
		UserID:    userID,
		Command:   command,
		Succeeded: execErr == nil,
		CreatedAt: now.UTC(),
	}
	if execErr != nil {
		run.Error = gameErrorMessage(execErr)
	} else {
		run.Result = result
	}
	return run
}
