package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// spoolBlockTimeout bounds a read that must not block on a FIFO. The reads under
// test either return at once or hang forever, so the bound only needs to be long.
const spoolBlockTimeout = 5 * time.Second

// verifiedStoreFor saves one completed session, confirms it through the spool and
// returns the store the host would read through, with that store's directory.
func verifiedStoreFor(t *testing.T, id string) (*FileSessionStore, string) {
	t.Helper()
	t.Setenv("HARNEZ_SPOOL_DIR", t.TempDir())
	storeDir := t.TempDir()
	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: id, Name: id, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	st, ok, err := VerifiedSpoolStore(id)
	if err != nil || !ok {
		t.Fatalf("VerifiedSpoolStore(%s): ok=%v err=%v", id, ok, err)
	}
	return st, storeDir
}

// A store opened from a verified spool entry must not follow a link that a
// confined writer puts in place after the check.
func TestSpoolBackedStoreDoesNotFollowSwappedEntry(t *testing.T) {
	st, storeDir := verifiedStoreFor(t, "sw1")
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"id":"sw1","name":"sw1","status":"completed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(storeDir, "sw1.json")
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, entry); err != nil {
		t.Fatal(err)
	}
	if sess, err := st.Get("sw1"); err == nil {
		t.Fatalf("Get followed a symlink swapped in after verification: session %q", sess.ID)
	}
}

// A registry entry over the pointer size cap is refused, as pointers are.
func TestSpoolBackedStoreRejectsOversizedEntry(t *testing.T) {
	st, storeDir := verifiedStoreFor(t, "big1")
	body := `{"id":"big1","name":"big1","status":"completed","response":"` + strings.Repeat("x", maxSpoolPointerBytes) + `"}`
	if err := os.WriteFile(filepath.Join(storeDir, "big1.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if sess, err := st.Get("big1"); err == nil {
		t.Fatalf("Get read an entry over the %d-byte cap: response of %d bytes", maxSpoolPointerBytes, len(sess.Response))
	}
}

// Each read re-checks the entry's session id. An entry that now names another
// session is not returned for the verified id, and Find must not fall back to
// listing the store, where that other session could match by name.
func TestSpoolBackedStoreRechecksSessionIDOnEachRead(t *testing.T) {
	st, storeDir := verifiedStoreFor(t, "rc1")
	body := `{"id":"other","name":"rc1","status":"completed"}`
	if err := os.WriteFile(filepath.Join(storeDir, "rc1.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if sess, err := st.Get("rc1"); err == nil {
		t.Fatalf("Get returned session %q for the verified id rc1", sess.ID)
	}
	if sess, err := st.Find("rc1"); err == nil {
		t.Fatalf("Find returned session %q after the entry changed identity", sess.ID)
	}
}

// A FIFO in place of an entry must fail the read at once, not block the host.
func TestSpoolBackedStoreDoesNotBlockOnFIFO(t *testing.T) {
	st, storeDir := verifiedStoreFor(t, "ff1")
	entry := filepath.Join(storeDir, "ff1.json")
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(entry, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := st.Get("ff1")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Get returned a session read from a FIFO")
		}
	case <-time.After(spoolBlockTimeout):
		releaseFIFO(entry, done)
		t.Fatal("Get blocked on a FIFO")
	}
}

// The bounded reader refuses a symlink and a FIFO on its own, so the spool and
// registry checks hold even if a path changes between Lstat and open.
func TestBoundedReadRefusesSymlinkAndFIFO(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if data, err := readBounded(link); err == nil {
		t.Fatalf("readBounded followed a symlink and returned %q", data)
	}
	fifo := filepath.Join(dir, "fifo.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readBounded(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("readBounded returned data from a FIFO")
		}
	case <-time.After(spoolBlockTimeout):
		releaseFIFO(fifo, done)
		t.Fatal("readBounded blocked on a FIFO")
	}
}

// A pointer whose file name differs from its session_id is forged, and a registry
// entry whose id differs from its file name is not confirmed. Neither verifies.
func TestSpoolMismatchedIDsAreNotVerified(t *testing.T) {
	t.Run("pointer names another session", func(t *testing.T) {
		spool := t.TempDir()
		t.Setenv("HARNEZ_SPOOL_DIR", spool)
		store, err := NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(&Session{ID: "pm1", Name: "pm1", Status: "running"}); err != nil {
			t.Fatal(err)
		}
		// Copy the pointer for pm1 under the file name pm2: its session_id is pm1.
		raw, err := os.ReadFile(filepath.Join(spool, "pm1.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(spool, "pm2.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		entries, err := ReadSpoolEntries(map[string]bool{})
		if err != nil {
			t.Fatal(err)
		}
		if e := spoolFor(t, entries, "pm2"); e.State == SpoolVerified {
			t.Fatal("pointer whose session_id differs from its file name was verified")
		}
		if _, ok, _ := VerifiedSpoolStore("pm2"); ok {
			t.Fatal("VerifiedSpoolStore opened a store for a mismatched pointer")
		}
	})
	t.Run("registry entry names another session", func(t *testing.T) {
		storeDir := t.TempDir()
		t.Setenv("HARNEZ_SPOOL_DIR", t.TempDir())
		store, err := NewSessionStore(storeDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(&Session{ID: "rm1", Name: "rm1", Status: "completed"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeDir, "rm1.json"), []byte(`{"id":"other","name":"rm1","status":"completed"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		entries, err := ReadSpoolEntries(map[string]bool{})
		if err != nil {
			t.Fatal(err)
		}
		if e := spoolFor(t, entries, "rm1"); e.State == SpoolVerified {
			t.Fatal("registry entry with another session id was verified")
		}
		if _, ok, _ := VerifiedSpoolStore("rm1"); ok {
			t.Fatal("VerifiedSpoolStore opened a store for a registry entry with another id")
		}
	})
}

// releaseFIFO opens the FIFO for writing so a reader blocked in open returns, then
// waits for that reader. It keeps a failed run from leaking a goroutine.
func releaseFIFO(path string, done <-chan error) {
	if w, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		_ = w.Close()
	}
	<-done
}
