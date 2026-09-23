package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type forceQuotaFetchKey struct{}

// ForceQuotaFetch marks a quota collection as requiring an upstream refresh.
// The shared live cache is still updated and used as an error fallback.
func ForceQuotaFetch(ctx context.Context) context.Context {
	return context.WithValue(ctx, forceQuotaFetchKey{}, true)
}

func quotaFetchForced(ctx context.Context) bool {
	forced, _ := ctx.Value(forceQuotaFetchKey{}).(bool)
	return forced
}

// liveFetchCacheFilename is the shared cross-process cache written next to
// each agent's own local cache/auth files (~/.claude, ~/.codex,
// ~/.gemini/antigravity-cli), so any `harnez` process (watch, one-shot
// `usage`, `--summary`) that reads live quota within MinWatchInterval of a
// sibling process's fetch reuses that reading instead of hitting the agent's
// live endpoint/RPC again. This is what stops concurrent `harnez` instances
// (or a --watch fallback racing the statecache daemon's own tick) from
// independently polling the same account and drawing errors or rate limits.
// Originally built for Claude alone (issue 033); generalized to Codex and
// AGY in issue 087.
const liveFetchCacheFilename = "harnez-quota-cache.json"

// liveFetchLockRetries/liveFetchLockDelay bound how long a writer waits for
// the advisory flock before giving up on persisting to disk. flock does not
// survive a hung holder, so this must never block indefinitely (issue 033) —
// a few short retries (~250ms total) is enough to let a sibling process's
// in-flight write finish without stalling this process's own response.
const liveFetchLockRetries = 5
const liveFetchLockDelay = 50 * time.Millisecond

// liveFetchCache is the generic on-disk shape of an agent's
// harnez-quota-cache.json: a fetch timestamp plus whatever quota payload
// shape that agent's live endpoint returns — Session/Weekly windows for
// Claude and Codex, ModelGroups for AGY.
type liveFetchCache[T any] struct {
	FetchedAt time.Time `json:"fetched_at"`
	Payload   T         `json:"payload"`
}

// liveFetchCachePath returns the shared cache path inside an agent's own
// config/state directory (e.g. ~/.claude, ~/.codex,
// ~/.gemini/antigravity-cli).
func liveFetchCachePath(agentDir string) string {
	return filepath.Join(agentDir, liveFetchCacheFilename)
}

// readLiveFetchCache reads and decodes the cache at path, returning nil if
// the file is missing or unparsable rather than an error the caller has to
// unwrap.
func readLiveFetchCache[T any](path string) *liveFetchCache[T] {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c liveFetchCache[T]
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	return &c
}

// lockLiveFetchCache takes a non-blocking, bounded-retry exclusive flock on a
// sidecar `.lock` file next to the cache. It is the mechanism recommended in
// issue 033 over a hand-rolled optimistic timestamp-check protocol: flock
// closes the write race completely and releases automatically on process
// exit (normal, crash, or kill) with no stale-lock bookkeeping. Returns
// ok=false if the lock isn't free within the retry budget — the caller must
// then skip the disk write rather than block, since flock does not detect a
// hung (not dead) holder.
func lockLiveFetchCache(path string) (f *os.File, ok bool) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false
	}
	for attempt := 0; attempt <= liveFetchLockRetries; attempt++ {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return f, true
		}
		if attempt < liveFetchLockRetries {
			time.Sleep(liveFetchLockDelay)
		}
	}
	f.Close()
	return nil, false
}

func unlockLiveFetchCache(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// liveFetchInProcessMutexes holds one *sync.Mutex per cache path, so
// concurrent goroutines within this single process (e.g. the statecache
// daemon's own tick racing a `--watch` fallback read, both landing on the
// same agent at once — the scenario issue 087 calls out) fully serialize on
// the check-cache/maybe-fetch/write sequence below instead of each reading a
// cold cache and firing off its own redundant live fetch. flock alone can't
// give this guarantee: it only ever protects the disk *write*, not the
// read-then-decide step before it, and two goroutines in the same process
// can each acquire it in turn. This does not coordinate across separate OS
// processes (still flock's job) — only within one.
var liveFetchInProcessMutexes sync.Map // path -> *sync.Mutex

// lockLiveFetchInProcess blocks until this process's own in-flight
// check/fetch/write for path (if any) finishes, then returns an unlock func
// the caller must defer. Pair with lockLiveFetchCache for the cross-process
// disk write lock; this one is intra-process only and never fails to
// acquire (it blocks rather than giving up).
func lockLiveFetchInProcess(path string) func() {
	v, _ := liveFetchInProcessMutexes.LoadOrStore(path, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// writeLiveFetchCache writes via the standard write-tmp-then-rename pattern
// so a concurrent reader can never observe a torn/partial write; rename is
// atomic on the same filesystem, so this alone would be enough for readers
// even without the lock — the lock exists to stop two writers interleaving.
func writeLiveFetchCache[T any](path string, cache liveFetchCache[T]) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
