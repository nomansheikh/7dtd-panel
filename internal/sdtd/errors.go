package sdtd

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is a structured failure reported by the game server.
//
// The server puts its machine-readable code in the response envelope's meta
// block rather than in the body, and may include a full .NET stack trace.
// Callers should surface Error to operators but never hand Trace to a
// browser; see SafeMessage.
type APIError struct {
	Status           int
	ErrorCode        string
	RequestMethod    string
	RequestSubpath   string
	ExceptionMessage string
	Trace            string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("game server returned %d", e.Status)
	if e.ErrorCode != "" {
		msg += " (" + e.ErrorCode + ")"
	}
	if e.ExceptionMessage != "" {
		msg += ": " + e.ExceptionMessage
	}
	return msg
}

// codeMessages turn the server's SCREAMING_SNAKE codes into something a person
// can read. The code itself is still reported separately, so nothing is lost.
var codeMessages = map[string]string{
	CodeUnknownCommand: "no such command on this server",
	CodeNoCommand:      "no command was given",
	CodeUnsupported:    "the server does not support this operation",
	CodeIDNotFound:     "not found",
	"INVALID_BODY":     "the server rejected the request body",
}

// SafeMessage is the real server error with the stack trace removed, suitable
// for showing to a logged-in operator. The brief is explicit that users should
// see the actual error rather than "something went wrong", so the server's own
// exception message is preferred and only the trace is dropped.
func (e *APIError) SafeMessage() string {
	if e.ExceptionMessage != "" {
		if e.ErrorCode != "" {
			return e.ErrorCode + ": " + e.ExceptionMessage
		}
		return e.ExceptionMessage
	}
	if e.ErrorCode != "" {
		if human, ok := codeMessages[e.ErrorCode]; ok {
			return human
		}
		return e.ErrorCode
	}
	if text := http.StatusText(e.Status); text != "" {
		return text
	}
	return "the game server returned an unexpected response"
}

// Known error codes the panel reacts to by name. The server defines many
// more; these are the ones worth distinguishing in the UI.
const (
	CodeUnknownCommand = "UNKNOWN_COMMAND"
	CodeNoCommand      = "NO_COMMAND"
	CodeUnsupported    = "Unsupported"
	CodeIDNotFound     = "ID_NOT_FOUND"
)

// ErrorCode returns the game server's error code for err, or "" if err is not
// an APIError.
func ErrorCode(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode
	}
	return ""
}

// IsNotFound reports whether err is a 404 from the game server.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// IsUnauthorized reports whether the API token was rejected. This usually
// means SDTD_API_TOKEN_NAME or SDTD_API_TOKEN_SECRET is wrong, or the token's
// permission level is too low for the endpoint.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) &&
		(apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden)
}
