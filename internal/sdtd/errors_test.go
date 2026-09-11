package sdtd

import (
	"net/http"
	"strings"
	"testing"
)

func TestSafeMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "exception message is preferred",
			err:  &APIError{Status: 500, ErrorCode: "INVALID_BODY", ExceptionMessage: "expected '{'"},
			want: "INVALID_BODY: expected '{'",
		},
		{
			name: "exception without a code",
			err:  &APIError{Status: 500, ExceptionMessage: "boom"},
			want: "boom",
		},
		{
			// A raw SCREAMING_SNAKE code is not a sentence a person can read.
			name: "known code becomes readable",
			err:  &APIError{Status: 404, ErrorCode: CodeUnknownCommand},
			want: "no such command on this server",
		},
		{
			name: "unknown code falls back to the code itself",
			err:  &APIError{Status: 400, ErrorCode: "SOME_NEW_CODE"},
			want: "SOME_NEW_CODE",
		},
		{
			name: "no code at all falls back to the status",
			err:  &APIError{Status: http.StatusForbidden},
			want: "Forbidden",
		},
		{
			name: "unrecognised status still says something",
			err:  &APIError{Status: 799},
			want: "the game server returned an unexpected response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.SafeMessage(); got != tt.want {
				t.Errorf("SafeMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSafeMessageNeverIncludesTheTrace(t *testing.T) {
	err := &APIError{
		Status: 500, ErrorCode: "INVALID_BODY",
		ExceptionMessage: "bad json", Trace: "at Webserver.WebAPI.Handle()",
	}
	if strings.Contains(err.SafeMessage(), "Webserver") {
		t.Error("SafeMessage leaked the stack trace")
	}
	if !strings.Contains(err.Error(), "bad json") {
		t.Error("Error() should still carry the server's message for logs")
	}
}
