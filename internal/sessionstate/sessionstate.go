// Package sessionstate implements a minimal per-session state store for
// harnez CLI invocations — see issue 183. Because harnez runs as a fresh
// process per invocation (unlike a long-running agent process), any
// cross-call memory of "what has this session already done with harnez"
// has to be persisted, not kept in memory.
//
// This deliberately reuses the existing per-session state directory
// convention from internal/resolve (~/.harnez/sessions/, already used for
// the session-id lock file and last-ticket history — see
// resolve.DefaultStateDir) rather than inventing a new location such as
// ~/.cache/harnez/sessions/. One JSON file per session, keyed by the same
// short-hash-of-session-id scheme resolve.go uses internally.
package sessionstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Invocation tracks how many times a given harnez subcommand has been
// invoked this session, and when it was last invoked.
type Invocation struct {
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
}

// State is one session's persisted harnez-usage history.
type State struct {
	SessionID string                `json:"session_id"`
	Total     int                   `json:"total"`
	Calls     map[string]Invocation `json:"calls"`

	// LastRateAt/TotalAtLastRate track the narrowed rate policy from issue
	// 181: a "gap" is calls since the last harnez rate call, not since the
	// last call of any kind.
	LastRateAt      time.Time `json:"last_rate_at,omitzero"`
	TotalAtLastRate int       `json:"total_at_last_rate"`

	// TotalAtLastTip rate-limits the reminder itself: a tip only fires
	// again once at least tipCooldown further calls have happened, so this
	// mechanism doesn't become the same chatty, ignored signal issue 181
	// narrowed harnez rate away from.
	TotalAtLastTip int `json:"total_at_last_tip"`

	// FirstCallAt records this session's first-ever harnez invocation time.
	// It's the wall-clock anchor issue 186's time-based gap tip uses when
	// LastRateAt is still zero (no `harnez rate` call yet this session) —
	// mirroring the count-based fallback that uses s.Total as callsSinceRate
	// in that same situation.
	FirstCallAt time.Time `json:"first_call_at,omitzero"`
}

// fileName returns the per-session state file's basename: a short hash of
// the session id, matching the scheme resolve.go's ticketHistoryPath uses
// for its own per-session file in the same directory, so the two features'
// files sit side by side without colliding.
func fileName(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:8]) + ".usage.json"
}

// Path returns the state file path for sessionID under stateDir (pass
// resolve.DefaultStateDir() in normal use; overridable for tests).
func Path(stateDir, sessionID string) string {
	return filepath.Join(stateDir, fileName(sessionID))
}

// Load reads a session's state, returning a fresh zero-value State (not an
// error) if no file exists yet — every session's first harnez call starts
// from a clean slate.
func Load(stateDir, sessionID string) (State, error) {
	s := State{SessionID: sessionID, Calls: map[string]Invocation{}}
	data, err := os.ReadFile(Path(stateDir, sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return State{SessionID: sessionID, Calls: map[string]Invocation{}}, nil //nolint:nilerr // corrupt state file: start fresh rather than fail the caller's real command
	}
	if s.Calls == nil {
		s.Calls = map[string]Invocation{}
	}
	return s, nil
}

// Save writes s to its state file under stateDir, creating the directory
// if needed.
func Save(stateDir string, s State) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(stateDir, s.SessionID), data, 0o644)
}

// Record updates s in place for one invocation of subcommand at time now.
func Record(s *State, subcommand string, now time.Time) {
	if s.Total == 0 && s.FirstCallAt.IsZero() {
		s.FirstCallAt = now
	}
	s.Total++
	inv := s.Calls[subcommand]
	inv.Count++
	inv.LastAt = now
	s.Calls[subcommand] = inv

	if subcommand == "rate" {
		s.LastRateAt = now
		s.TotalAtLastRate = s.Total
	}
}

// Tuning constants for the gap heuristics. Deliberately simple (fixed
// call-count thresholds, no time-based decay) for a v1 — see issue 183.
// Issue 186 adds a wall-clock counterpart (rateGapIdle) alongside these.
const (
	// rateGapThreshold is how many harnez calls may pass since the last
	// `harnez rate` call (a failure rating OR an --ok heartbeat — Record
	// treats both the same, see its doc comment) before a gap tip becomes
	// eligible to fire.
	rateGapThreshold = 20
	// heartbeatGapThreshold is how many calls may pass since the last
	// `harnez rate` call before the gap tip switches from the generic
	// "you haven't rated anything" reminder to specifically suggesting the
	// lean `harnez rate --ok` heartbeat (issue 179). Set to 2x
	// rateGapThreshold so it only takes over once a full reminder cycle has
	// already gone unanswered — this stays rarer than the existing tip
	// rather than adding a second frequent nag, per issue 179's "don't
	// regress to pre-181 chattiness" constraint.
	heartbeatGapThreshold = 2 * rateGapThreshold
	// findMinCalls is the minimum total call count before "you've never
	// used harnez find" becomes a meaningful observation rather than noise
	// on a session's very first call.
	findMinCalls = 8
	// tipCooldown is the minimum number of further calls between two tips,
	// so a persistent gap doesn't nag on every single invocation.
	tipCooldown = 10

	// rateGapIdle is issue 186's wall-clock counterpart to rateGapThreshold:
	// a session with few but widely-spaced-out calls may never cross the
	// call-count threshold, so idle time alone can also make the plain
	// rate-gap tip eligible. 20 minutes is a starting default — long enough
	// that a normal think-then-act pause doesn't trip it, short enough to
	// still catch a genuinely idle/forgotten session within one sitting.
	// Overridable via rateGapIdleEnv for tuning without a code change.
	rateGapIdle = 20 * time.Minute

	// rateGapIdleEnv, when set to a positive integer, overrides rateGapIdle
	// (in minutes). heartbeatGapIdle is always derived as 2x whatever value
	// is in effect, mirroring heartbeatGapThreshold's relationship to
	// rateGapThreshold.
	rateGapIdleEnv = "HARNEZ_RATE_GAP_IDLE_MINUTES"
)

// rateGapIdleDuration returns the effective wall-clock idle threshold for
// the plain rate-gap tip: rateGapIdle, unless rateGapIdleEnv names a
// positive integer number of minutes.
func rateGapIdleDuration() time.Duration {
	if v := os.Getenv(rateGapIdleEnv); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return rateGapIdle
}

// UnratedFailures returns the number of genuinely-failed tool calls this
// session recorded (per internal/telemetry's GroupStats.FailureCount
// definition: exit_code != 0 OR score <= 2) that have gone unrated since
// the last `harnez rate` call — see internal/telemetry's
// UnratedFailureCount, which sessionTipHook queries and passes in here.
// Issue 226: a shell command the agent ran expecting it to fail (see
// cmd/harnez/exec.go's HARNEZ_EXPECT_FAILURE convention) is excluded from
// this count at the telemetry layer (UnratedFailureCount itself skips
// telemetry.ExpectedFailureCallType rows) — GapTip below never has to know
// about that distinction, it just sees a smaller/zero unratedFailures.
// sessionstate itself never talks to internal/telemetry directly (it has
// no DB dependency and stays a pure, easily-unit-tested package); the
// count crossing this package's boundary as a plain int keeps that
// separation while still letting GapTip prioritize it correctly.
//
// GapTip returns at most one short, single-line proactive tip given s's
// current state as of now, or ok=false if nothing is worth surfacing right
// now (either no gap is detected, or a tip already fired within tipCooldown
// calls). unratedFailures is issue 188's sharper signal: when positive (and
// feedback isn't disabled), it takes priority over every other tip below —
// "N real failures went unrated" is strictly more actionable than the
// generic count/time-based silence tips, so it preempts them rather than
// stacking alongside them in the same call. It still respects the same
// tipCooldown gate as everything else in this function, so a persistent
// unrated-failure streak doesn't nag on every single invocation either.
// feedbackDisabled mirrors claude.RateFeedbackDisabled (issue 142):
// when true, both rate-related tips (the failure-rating reminder and the
// issue-179 heartbeat nudge) are suppressed — a session that opted out of
// the Tool Feedback Protocol shouldn't get nagged about either half of it —
// though the unrelated find-underuse tip still can fire. Checking order is
// deliberate: the larger heartbeat gap is checked before the plain rate
// gap (more specific/actionable once the silence has gone on long enough),
// which in turn takes priority over the find-underuse observation.
//
// Issue 186: alongside the call-count gap (callsSinceRate), each threshold
// also has a wall-clock counterpart (elapsedSinceRate vs rateGapIdle /
// heartbeatGapIdle) — a slow-paced session with few calls per unit time may
// never cross the count threshold even though a long silence has passed.
// Whichever signal (count or time) crosses its threshold first wins; they
// share the same tipCooldown gate and feedbackDisabled opt-out rather than
// firing as a separate, redundant tip.
//
// Issue 187: a third, spec-defined ("summary") reminder composes into this
// same slot rather than adding a fourth independent nag channel: once the
// gap crosses spec/reminders.yaml's summary.threshold_multiplier (checked
// before the heartbeat/plain thresholds, since it only fires at a strictly
// larger gap than either), its concrete "X calls in <dur>" wording replaces
// what the heartbeat or plain tip would otherwise have said this call. See
// summaryReminder()/renderReminder() in remindersspec.go.
func GapTip(s State, feedbackDisabled bool, now time.Time, unratedFailures int) (string, bool) {
	if s.Total-s.TotalAtLastTip < tipCooldown {
		return "", false
	}

	if !feedbackDisabled && unratedFailures > 0 {
		return fmt.Sprintf("harnez tip: %d tool call(s) failed without a `harnez rate` "+
			"report — use `harnez rate <tool> <score> \"<summary>\"` to record what went "+
			"wrong (see Tool Feedback Protocol).", unratedFailures), true
	}

	if !feedbackDisabled {
		callsSinceRate := s.Total - s.TotalAtLastRate
		rateAnchor := s.LastRateAt
		if rateAnchor.IsZero() {
			callsSinceRate = s.Total
			rateAnchor = s.FirstCallAt
		}
		var elapsedSinceRate time.Duration
		if !rateAnchor.IsZero() && now.After(rateAnchor) {
			elapsedSinceRate = now.Sub(rateAnchor)
		}
		rateIdle := rateGapIdleDuration()
		heartbeatIdle := 2 * rateIdle

		// Issue 187: the spec-defined "summary" reminder gives a concrete
		// call count + duration instead of the plain/heartbeat tips' vaguer
		// wording, but only once the gap has gone on long enough that both
		// of those have already had a chance to fire and be ignored (its
		// threshold_multiplier is checked first, ahead of the heartbeat and
		// plain thresholds below, since it fires only at a strictly larger
		// gap than either). It composes into this same one-tip-per-call
		// slot rather than stacking as a third nag channel: whichever
		// condition matches, GapTip returns at most one line. A missing or
		// invalid embedded spec just skips this branch, falling back to the
		// plain/heartbeat tips below.
		if r, ok := summaryReminder(); ok {
			summaryThreshold := int(r.ThresholdMultiplier * float64(rateGapThreshold))
			summaryIdle := time.Duration(r.ThresholdMultiplier * float64(rateIdle))
			if callsSinceRate >= summaryThreshold || elapsedSinceRate >= summaryIdle {
				return renderReminder(r, callsSinceRate, elapsedSinceRate), true
			}
		}

		if callsSinceRate >= heartbeatGapThreshold || elapsedSinceRate >= heartbeatIdle {
			return fmt.Sprintf("harnez tip: %d+ calls since any `harnez rate` call this "+
				"session — if nothing has failed, confirm with `harnez rate --ok` instead "+
				"of staying silent (see Tool Feedback Protocol).", heartbeatGapThreshold), true
		}
		if callsSinceRate >= rateGapThreshold || elapsedSinceRate >= rateIdle {
			return "harnez tip: no `harnez rate` call in this session's last " +
				"20+ calls — remember, only rate a tool call that failed or " +
				"missed the expected outcome (see Tool Feedback Protocol).", true
		}
	}

	if s.Total >= findMinCalls {
		if inv, ok := s.Calls["find"]; !ok || inv.Count == 0 {
			return "harnez tip: this session hasn't used `harnez find` yet — " +
				"use it instead of `ls issues/`/grep for ticket discovery.", true
		}
	}

	return "", false
}
