//go:build !linux

package main

import "errors"

func socketPeerPID(string) (int, error) {
	return 0, errors.New("Unix socket peer credentials are unavailable")
}
