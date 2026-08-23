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
