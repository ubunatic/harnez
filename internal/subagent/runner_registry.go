package subagent

import (
	"context"

	"ubunatic.com/harnez/internal/server"
)

// RegisterOwnSession registers the runner's launch-bound session via the
// inherited host broker FD without directly opening ~/.harnez/agents.
func RegisterOwnSession(ctx context.Context, req server.RunnerRegistryRequest) (*Session, error) {
	return callRunnerRegistry(ctx, server.RunnerOpOwnRegister, req)
}

// UpdateOwnSession updates the runner's launch-bound session fields via the
// inherited host broker FD. Cross-session IDs, forged paths, forged PIDs, and
// legacy records without launch identities are rejected by the broker.
func UpdateOwnSession(ctx context.Context, req server.RunnerRegistryRequest) (*Session, error) {
	return callRunnerRegistry(ctx, server.RunnerOpOwnUpdate, req)
}

// QueryOwnSession fetches only the caller's launch-bound session record via
// the inherited host broker FD.
func QueryOwnSession(ctx context.Context, sessionID string) (*Session, error) {
	return callRunnerRegistry(ctx, server.RunnerOpOwnQuery, server.RunnerRegistryRequest{
		SessionID: sessionID,
	})
}

func callRunnerRegistry(ctx context.Context, op string, req server.RunnerRegistryRequest) (*Session, error) {
	cfg, err := server.LoadConfig()
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := server.SendRunnerRegistry(ctx, cfg, op, req, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}
