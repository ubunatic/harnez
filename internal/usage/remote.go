package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// remoteUsagePayload represents the JSON payload returned by `harnez usage --json [--proc]`.
type remoteUsagePayload struct {
	UsageSummary
	Processes *AgentProcessCount `json:"processes,omitempty"`
}

// CollectRemote queries usage metrics from a remote host over SSH using `harnez usage --json [--proc]`.
// If SSH fails or harnez is unavailable remotely, it returns a fallback UsageSummary with
// QuotaFetchError set, along with the error.
func CollectRemote(ctx context.Context, host string, includeProcs bool) (UsageSummary, *AgentProcessCount, error) {
	return collectRemote(ctx, host, includeProcs, nil)
}

// CollectRemoteProgress is CollectRemote with an additional FetchProgressFunc
// (issue 169). Unlike CollectAllProgress's real per-source fan-out, a remote
// fetch is one opaque SSH round trip (there is no per-source breakdown to
// observe from here), so it reports exactly one FetchStarted immediately
// followed by one FetchDone/FetchFailed once the SSH call returns -- enough
// for the `--watch` startup splash to show "fetching <host>..." while the
// call is in flight. progress may be nil (same behavior as CollectRemote).
func CollectRemoteProgress(ctx context.Context, host string, includeProcs bool, progress FetchProgressFunc) (UsageSummary, *AgentProcessCount, error) {
	return collectRemote(ctx, host, includeProcs, progress)
}

func collectRemote(ctx context.Context, host string, includeProcs bool, progress FetchProgressFunc) (UsageSummary, *AgentProcessCount, error) {
	if progress != nil {
		progress(host, FetchStarted)
	}
	summary, procs, err := doCollectRemote(ctx, host, includeProcs)
	if progress != nil {
		if err != nil {
			progress(host, FetchFailed)
		} else {
			progress(host, FetchDone)
		}
	}
	return summary, procs, err
}

func doCollectRemote(ctx context.Context, host string, includeProcs bool) (UsageSummary, *AgentProcessCount, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		err := fmt.Errorf("ssh host cannot be empty")
		return makeRemoteFallbackSummary(host, err), nil, err
	}
	if !validSSHHostRe.MatchString(host) || strings.HasPrefix(host, "-") {
		err := fmt.Errorf("invalid ssh host %q", host)
		return makeRemoteFallbackSummary(host, err), nil, err
	}

	remoteCmd := `PATH="$PATH:$HOME/go/bin:$HOME/bin:/usr/local/bin" harnez usage --json`
	if includeProcs {
		remoteCmd += ` --proc`
	}

	// ssh -q -o BatchMode=yes -o ConnectTimeout=5 <host> <command>
	cmd := exec.CommandContext(ctx, "ssh", "-q", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", host, remoteCmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		sshErr := fmt.Errorf("remote query to %s failed: %s", host, errMsg)
		return makeRemoteFallbackSummary(host, sshErr), nil, sshErr
	}

	summary, procCounts, err := parseRemoteJSON(stdout.Bytes())
	if err != nil {
		parseErr := fmt.Errorf("failed to parse remote JSON from %s: %w", host, err)
		return makeRemoteFallbackSummary(host, parseErr), nil, parseErr
	}

	return summary, procCounts, nil
}

// CollectRemoteLoadSnapshot fetches just the CPU/GPU load reading for host
// via one plain batch SSH call (issue 110 Decision §2: every mode other
// than `--watch` always uses a single stateless `ssh host "harnez usage
// --json"`-style call, no ControlMaster, no persistence). It's the
// --summary/plain-mode counterpart of StartRemoteLoadStream's --watch-only
// streaming path — both ultimately feed the same "[R] Remote Load" panel
// (buildRemoteLoadBox), just via different collection mechanisms.
//
// A nil snapshot (with the underlying error, for callers that want it) is
// returned on any failure — the panel already renders an explicit "remote
// load unavailable" placeholder for a nil snapshot, matching how a stale/
// failed --host fetch is already handled.
func CollectRemoteLoadSnapshot(ctx context.Context, host string) (*LoadSnapshot, error) {
	summary, _, err := CollectRemote(ctx, host, false)
	if err != nil {
		return nil, err
	}
	return summary.Load, nil
}

// parseRemoteJSON parses raw JSON output from `harnez usage --json [--proc]`.
func parseRemoteJSON(data []byte) (UsageSummary, *AgentProcessCount, error) {
	// First try unmarshaling into remoteUsagePayload (which supports both UsageSummary and Processes)
	var payload remoteUsagePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return UsageSummary{}, nil, err
	}

	// If timestamp is zero, set current timestamp
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}

	return payload.UsageSummary, payload.Processes, nil
}

// makeRemoteFallbackSummary constructs a fallback UsageSummary when SSH or remote execution fails.
func makeRemoteFallbackSummary(host string, err error) UsageSummary {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return UsageSummary{
		Timestamp: time.Now(),
		Agents: []AgentUsage{
			{
				AgentID:         "remote",
				Name:            fmt.Sprintf("Remote (%s)", host),
				Installed:       true,
				Authenticated:   false,
				QuotaFetchError: errStr,
				Error:           errStr,
			},
		},
	}
}
