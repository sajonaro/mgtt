package probe

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors form the typed error taxonomy returned by ExternalRunner.
// Exit codes from the provider runner map to these per the probe protocol
// (see docs/PROBE_PROTOCOL.md). Callers use errors.Is to branch.
var (
	ErrUsage     = errors.New("provider: usage error")
	ErrEnv       = errors.New("provider: environment error")
	ErrForbidden = errors.New("provider: forbidden")
	ErrTransient = errors.New("provider: transient error")
	ErrProtocol  = errors.New("provider: protocol error")
	ErrUnknown   = errors.New("provider: unknown error")
)

// ClassifyExit maps a runner exit code + stderr line to a sentinel error.
// See docs/PROBE_PROTOCOL.md for the canonical mapping.
func ClassifyExit(code int, stderr string) error {
	msg := firstLine(stderr)
	switch code {
	case 1:
		return fmt.Errorf("%w: %s", ErrUsage, msg)
	case 2:
		return fmt.Errorf("%w: %s", ErrEnv, msg)
	case 3:
		return fmt.Errorf("%w: %s", ErrForbidden, msg)
	case 4:
		return fmt.Errorf("%w: %s", ErrTransient, msg)
	case 5:
		return fmt.Errorf("%w: %s", ErrProtocol, msg)
	}
	return fmt.Errorf("%w: exit %d: %s", ErrUnknown, code, msg)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
