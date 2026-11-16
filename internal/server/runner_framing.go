package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"
)

func validateConnectedUnixSocket(fd, wantType int) error {
	domain, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_DOMAIN)
	if err != nil || domain != syscall.AF_UNIX {
		return fmt.Errorf("not an AF_UNIX socket")
	}
	sockType, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil || sockType != wantType {
		return fmt.Errorf("unexpected socket type")
	}
	if _, err := syscall.Getpeername(fd); err != nil {
		return fmt.Errorf("not connected: %w", err)
	}
	return nil
}

func writeRunnerResponse(w io.Writer, maxBytes int64, resp RunnerResponse) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return writeChannelFrame(w, raw, maxBytes)
}

func writeChannelFrame(w io.Writer, body []byte, maxBytes int64) error {
	if maxBytes <= 0 || int64(len(body)) > maxBytes {
		return ErrRunnerPayloadTooLarge
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func readChannelFrame(r io.Reader, maxBytes int64) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || maxBytes <= 0 || int64(n) > maxBytes {
		return nil, ErrRunnerPayloadTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func decodeStrictJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON data")
	}
	return nil
}
