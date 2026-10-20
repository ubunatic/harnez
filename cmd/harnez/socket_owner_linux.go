//go:build linux

package main

import (
	"fmt"
	"net"
	"syscall"
)

func socketPeerPID(path string) (int, error) {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var credentials *syscall.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		credentials, socketErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if socketErr != nil {
		return 0, socketErr
	}
	if credentials == nil || credentials.Pid <= 0 {
		return 0, fmt.Errorf("Unix socket peer did not provide a process ID")
	}
	return int(credentials.Pid), nil
}
