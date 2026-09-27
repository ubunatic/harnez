//go:build !unix

package usage

import "context"

func startIfAbsent(Options) error { return ErrControllerUnsupported }

func requestController(context.Context, Options, wireRequest) (wireResponse, error) {
	return wireResponse{}, ErrControllerUnsupported
}

func subscribeController(context.Context, Options, ProviderID) (<-chan SnapshotEvent, error) {
	return nil, ErrControllerUnsupported
}

func closeController(*controller) {}
