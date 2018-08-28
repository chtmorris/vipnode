package pool

import (
	"fmt"
	"strings"
)

// ErrNoHostNodes is returned when the pool does not have any hosts available.
type ErrNoHostNodes struct {
	NumTried int
}

func (err ErrNoHostNodes) Error() string {
	if err.NumTried == 0 {
		return "no host nodes available"
	}
	return fmt.Sprintf("no available host nodes found after trying %d nodes", err.NumTried)
}

// ErrVerifyFailed is returned when a signature fails to verify. It embeds
// the underlying Cause.
type ErrVerifyFailed struct {
	Cause  error
	Method string
}

func (err ErrVerifyFailed) Error() string {
	return fmt.Sprintf("method %q failed to verify signature: %s", err.Method, err.Cause)
}

// ErrConnectFailed is returned when connect fails to
// whitelist the client on remote hosts.
type ErrConnectFailed struct {
	Errors []error
}

func (err ErrConnectFailed) Error() string {
	if len(err.Errors) == 0 {
		return "no host connection errors"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "failed to connect to %d hosts: ", len(err.Errors))
	for i, e := range err.Errors {
		s.WriteString(e.Error())
		if i != len(err.Errors)-1 {
			s.WriteString("; ")
		}
	}
	return s.String()
}
