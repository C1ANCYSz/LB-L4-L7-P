package l4

import (
	"lb-go/resources"
	"log/slog"
)

func (lb *LoadBalancer) handleConnError(direction string, err error, kind ConnErrKind, server *resources.Backend) {

	switch kind {
	case ErrKindNone, ErrKindBenign, ErrKindCancelled:
		// expected, swallow
	case ErrKindTimeout:
		slog.Warn("connection timed out",
			slog.String("direction", direction),
			slog.Any("err", err),
		)
	case ErrKindRefused:
		// backend is down — mark it
		server.Up.Store(false)
		slog.Error("backend refused connection",
			slog.String("direction", direction),
			slog.String("server", *server.Address.Load()),
			slog.Any("err", err),
		)
		go lb.ResolveBackend(server)
	case ErrKindUnreachable:
		server.Up.Store(false)
		slog.Error("backend unreachable",
			slog.String("direction", direction),
			slog.String("server", *server.Address.Load()),
			slog.Any("err", err),
		)
		go lb.ResolveBackend(server)
	case ErrKindExhausted:
		// this is a system-level emergency
		slog.Error("RESOURCE EXHAUSTION — file descriptors or memory",
			slog.String("direction", direction),
			slog.Any("err", err),
		)
	default:
		// ErrKindUnknown — log everything, don't swallow
		slog.Error("unclassified connection error",
			slog.String("direction", direction),
			slog.String("kind", kind.String()),
			slog.Any("err", err),
		)
	}
}
