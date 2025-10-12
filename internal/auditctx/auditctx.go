package auditctx

import "context"

// Actor captures contextual information about the authenticated actor that initiated a request.
type Actor struct {
	UserID    string
	Username  string
	IPAddress string
	UserAgent string
}

type actorContextKey struct{}

// WithActor injects actor metadata into the supplied context, returning a derived context that
// callers can pass down into service layers for audit logging.
func WithActor(ctx context.Context, actor Actor) context.Context {
	if ctx == nil {
		return context.WithValue(context.Background(), actorContextKey{}, actor)
	}
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// FromContext extracts previously stored actor metadata from the context.
func FromContext(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}
