package l4

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
)

type ConnErrKind int

const (
	ErrKindNone        ConnErrKind = iota // not an err
	ErrKindBenign                         // clean close, peer reset, or a broken pipe
	ErrKindTimeout                        // idle or dial timeout
	ErrKindRefused                        // resource actively refused
	ErrKindUnreachable                    // no route, network down
	ErrKindExhausted                      // fd limit, too many open files
	ErrKindCancelled                      // cancelled by the context in handleConn()
	ErrKindUnknown                        // unknown errs
)

func (k ConnErrKind) String() string {
	switch k {
	case ErrKindNone:
		return "none"
	case ErrKindBenign:
		return "benign"
	case ErrKindTimeout:
		return "timeout"
	case ErrKindRefused:
		return "refused"
	case ErrKindUnreachable:
		return "unreachable"
	case ErrKindExhausted:
		return "exhausted"
	case ErrKindCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

func classifyConnError(err error) ConnErrKind {
	if err == nil {
		return ErrKindNone
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrKindCancelled
	}

	// clean close from either side
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ErrKindBenign
	}

	// net.Error covers timeout and temporary errors
	if netErr, ok := errors.AsType[net.Error](err); ok {
		if netErr.Timeout() {
			return ErrKindTimeout
		}
	}

	// syscall-level errors — most precise classification
	if syscallErr, ok := errors.AsType[*os.SyscallError](err); ok {
		inner := syscallErr.Unwrap()
		switch {
		case errors.Is(inner, syscall.ECONNREFUSED):
			return ErrKindRefused
		case errors.Is(inner, syscall.ECONNRESET):
			return ErrKindBenign // peer reset mid-transfer, normal
		case errors.Is(inner, syscall.EPIPE):
			return ErrKindBenign // broken pipe, client gone
		case errors.Is(inner, syscall.ENETUNREACH),
			errors.Is(inner, syscall.EHOSTUNREACH),
			errors.Is(inner, syscall.ENETDOWN):
			return ErrKindUnreachable
		case errors.Is(inner, syscall.EMFILE),
			errors.Is(inner, syscall.ENFILE),
			errors.Is(inner, syscall.ENOMEM):
			return ErrKindExhausted
		case errors.Is(inner, syscall.ETIMEDOUT):
			return ErrKindTimeout
		}
	}

	// op errors wrap the above on some platforms
	if opErr, ok := errors.AsType[*net.OpError](err); ok {
		inner := opErr.Unwrap()
		// recurse on the inner error
		kind := classifyConnError(inner)
		if kind != ErrKindUnknown {
			return kind
		}
	}

	// last resort string matching for windows and other platform quirks
	msg := err.Error()
	switch {
	case strings.Contains(msg, "forcibly closed by the remote host"), // windows ECONNRESET
		strings.Contains(msg, "connection reset by peer"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "use of closed network connection"),
		errors.Is(err, net.ErrClosed):
		return ErrKindBenign

	case strings.Contains(msg, "connection refused"):
		return ErrKindRefused

	case strings.Contains(msg, "no route to host"),
		strings.Contains(msg, "network is unreachable"):
		return ErrKindUnreachable

	case strings.Contains(msg, "too many open files"):
		return ErrKindExhausted

	case strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "deadline exceeded"):
		return ErrKindTimeout

	case strings.Contains(msg, "wsasend"),
		strings.Contains(msg, "wsarecv"),
		strings.Contains(msg, "aborted by the software in your host machine"):
		return ErrKindBenign
	}

	return ErrKindUnknown
}
