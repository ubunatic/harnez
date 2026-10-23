package main

import (
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// A socket provides a readiness event across exec without polling files or
// sleeping. The deadline is only a watchdog for a genuinely stuck child.
func processTestListener(t *testing.T) *net.TCPListener {
	t.Helper()
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func acceptProcessTestEvent(t *testing.T, l *net.TCPListener) net.Conn {
	t.Helper()
	_ = l.SetDeadline(time.Now().Add(5 * time.Second * raceFactor))
	c, err := l.Accept()
	if err != nil {
		t.Fatalf("child readiness event: %v", err)
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second * raceFactor))
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func releaseProcessTestChild(t *testing.T, l *net.TCPListener) {
	t.Helper()
	c := acceptProcessTestEvent(t, l)
	if _, err := c.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func awaitProcessTestRelease(t *testing.T) {
	t.Helper()
	c, err := net.DialTimeout("tcp4", os.Getenv("HARNEZ_TEST_PROCESS_GATE"), 5*time.Second*raceFactor)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second * raceFactor))
	if _, err := io.ReadFull(c, make([]byte, 1)); err != nil {
		t.Fatalf("parent release event: %v", err)
	}
}
