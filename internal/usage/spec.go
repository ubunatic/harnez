package usage

import (
	"bytes"
	"fmt"
	"io/fs"
	"math"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const usageSpecPath = "spec/usage.yaml"

type usageSpec struct {
	DebugLabels              usageDebugLabels   `yaml:"debug_labels"`
	AGYQuota                 agyQuotaSpec       `yaml:"agy_quota"`
	WatchLoadingLabel        string             `yaml:"watch_loading_label"`
	AGYMeterMaxAge           string             `yaml:"agy_meter_max_age"`
	CollectorCadence         string             `yaml:"collector_cadence"`
	CacheStalenessMultiplier yaml.Node          `yaml:"cache_staleness_multiplier"`
	DisplayStaleness         string             `yaml:"display_staleness"`
	AgentQuotaWarnPercent    float64            `yaml:"agent_quota_warn_percent"`
	AgentQuotaMaxReadingAge  string             `yaml:"agent_quota_max_reading_age"`
	CollectorTimeout         string             `yaml:"collector_timeout"`
	PassiveDedupeInterval    string             `yaml:"passive_dedupe_interval"`
	PassiveUpdatePath        string             `yaml:"passive_update_path"`
	UsageStoreBusyTimeout    string             `yaml:"usage_store_busy_timeout"`
	HistorySnapshotEvery     string             `yaml:"history_snapshot_interval"`
	ExportHostKey            string             `yaml:"export_host_key"`
	ClientHeartbeatInterval  string             `yaml:"client_heartbeat_interval"`
	ClientTimeout            string             `yaml:"client_timeout"`
	CatchupFetchThreshold    string             `yaml:"catchup_fetch_threshold"`
	NVIDIASMITimeout         string             `yaml:"nvidia_smi_timeout"`
	NVIDIASMICacheTTL        string             `yaml:"nvidia_smi_cache_ttl"`
	RemoteCommandTimeout     string             `yaml:"remote_command_timeout"`
	SSHConnectTimeout        string             `yaml:"ssh_connect_timeout"`
	RemoteLoadPollAttached   string             `yaml:"remote_load_poll_interval_attached"`
	RemoteLoadPollDetached   string             `yaml:"remote_load_poll_interval_detached"`
	RemoteLoadStaleness      string             `yaml:"remote_load_staleness"`
	UsageKillLog             usageKillLogSpec   `yaml:"usage_kill_log"`
	ViewModes                usageViewModesSpec `yaml:"view_modes"`
}

type usageKillLogSpec struct {
	Path        string `yaml:"path"`
	MaxBytes    int64  `yaml:"max_bytes"`
	Backups     int    `yaml:"backups"`
	CheckBudget string `yaml:"check_budget"`
}

func usageKillLogSettings() (usageKillLogSpec, time.Duration, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		return usageKillLogSpec{}, 0, err
	}
	var spec usageSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return usageKillLogSpec{}, 0, err
	}
	killLog := spec.UsageKillLog
	if killLog.Path == "" || killLog.MaxBytes <= 0 || killLog.Backups < 0 {
		return usageKillLogSpec{}, 0, fmt.Errorf("%s: usage_kill_log path, positive max_bytes, and non-negative backups are required", usageSpecPath)
	}
	budget, err := time.ParseDuration(killLog.CheckBudget)
	if err != nil || budget <= 0 {
		return usageKillLogSpec{}, 0, fmt.Errorf("%s: usage_kill_log.check_budget must be a positive duration", usageSpecPath)
	}
	return killLog, budget, nil
}

type agyQuotaSpec struct {
	DefaultPool string `yaml:"default_pool"`
	ModelPools  []struct {
		Contains []string `yaml:"contains"`
		Pool     string   `yaml:"pool"`
	} `yaml:"model_pools"`
	MeterBuckets []struct {
		Bucket string `yaml:"bucket"`
		Group  string `yaml:"group"`
		Window string `yaml:"window"`
	} `yaml:"meter_buckets"`
}

func agyQuotaSettings() (agyQuotaSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		return agyQuotaSpec{}, err
	}
	var spec usageSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return agyQuotaSpec{}, err
	}
	if spec.AGYQuota.DefaultPool == "" || len(spec.AGYQuota.ModelPools) == 0 || len(spec.AGYQuota.MeterBuckets) == 0 {
		return agyQuotaSpec{}, fmt.Errorf("%s: agy_quota settings are required", usageSpecPath)
	}
	return spec.AGYQuota, nil
}

type usageViewModesSpec struct {
	Default string                       `yaml:"default"`
	Modes   map[string]usageViewModeSpec `yaml:"modes"`
}

type usageViewModeSpec struct {
	Panels       []string `yaml:"panels"`
	TokenDetails *bool    `yaml:"token_details"`
	TitleBar     *bool    `yaml:"title_bar"`
	StatusBar    *bool    `yaml:"status_bar"`
	HiddenList   *bool    `yaml:"hidden_list"`
	OverflowHint *bool    `yaml:"overflow_hint"`
}

func (s usageViewModeSpec) tokenDetails() bool { return s.TokenDetails != nil && *s.TokenDetails }
func (s usageViewModeSpec) titleBar() bool     { return s.TitleBar != nil && *s.TitleBar }
func (s usageViewModeSpec) statusBar() bool    { return s.StatusBar != nil && *s.StatusBar }
func (s usageViewModeSpec) hiddenList() bool   { return s.HiddenList != nil && *s.HiddenList }
func (s usageViewModeSpec) overflowHint() bool { return s.OverflowHint != nil && *s.OverflowHint }

var (
	usageSpecOnce                 sync.Once
	usageSpecAge                  time.Duration
	usageSpecCadence              time.Duration
	usageSpecTimeout              time.Duration
	usageSpecDedupe               time.Duration
	usageSpecPassivePath          string
	usageSpecStoreBusyTimeout     time.Duration
	usageSpecErr                  error
	usageSpecLoadingLabel         string
	usageSpecHistoryEvery         time.Duration
	usageSpecHostKey              string
	usageSpecHeartbeatEvery       time.Duration
	usageSpecClientTimeout        time.Duration
	usageSpecCatchupThreshold     time.Duration
	usageSpecViewModes            usageViewModesSpec
	usageSpecNVTimeout            time.Duration
	usageSpecNVCacheTTL           time.Duration
	usageSpecRemoteCommandTimeout time.Duration
	usageSpecSSHConnectTimeout    time.Duration
	usageSpecRemotePollAttached   time.Duration
	usageSpecRemotePollDetached   time.Duration
	usageSpecRemoteStaleness      time.Duration
	usageSpecCachePolicy          cacheFreshnessPolicy
)

type cacheFreshnessPolicy struct {
	cadence, cacheStaleness, displayStaleness time.Duration
}

func (s usageSpec) cachePolicy() (cacheFreshnessPolicy, error) {
	cadence, err := time.ParseDuration(s.CollectorCadence)
	if err != nil || cadence <= 0 {
		return cacheFreshnessPolicy{}, fmt.Errorf("%s: collector_cadence must be a positive duration", usageSpecPath)
	}
	var multiplier int64
	if s.CacheStalenessMultiplier.Tag != "!!int" || s.CacheStalenessMultiplier.Decode(&multiplier) != nil || multiplier <= 0 || multiplier > math.MaxInt64/int64(cadence) {
		return cacheFreshnessPolicy{}, fmt.Errorf("%s: cache_staleness_multiplier must be positive and fit within a duration", usageSpecPath)
	}
	display, err := time.ParseDuration(s.DisplayStaleness)
	if err != nil || display <= 0 {
		return cacheFreshnessPolicy{}, fmt.Errorf("%s: display_staleness must be a positive duration", usageSpecPath)
	}
	return cacheFreshnessPolicy{cadence, cadence * time.Duration(multiplier), display}, nil
}

// Embedded specs are application code: invalid policy is a build defect,
// never a reason to silently substitute different freshness thresholds.
func mustCacheFreshnessPolicy() cacheFreshnessPolicy {
	if _, _, _, _, _, err := usageDurations(); err != nil {
		panic(err)
	}
	return usageSpecCachePolicy
}

func agyMeterMaxAge() (time.Duration, error) {
	age, _, _, _, _, err := usageDurations()
	return age, err
}

func collectorPolicy() (time.Duration, time.Duration, error) {
	_, cadence, timeout, _, _, err := usageDurations()
	return cadence, timeout, err
}

// historySnapshotInterval is the minimum gap between two usage-history
// snapshots that live collection records for one host (issue 735).
func historySnapshotInterval() (time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecHistoryEvery, err
}

// nvidiaSMIPolicy returns the nvidia-smi call timeout and how long one
// reading is reused before nvidia-smi runs again (issue 765).
func nvidiaSMIPolicy() (time.Duration, time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecNVTimeout, usageSpecNVCacheTTL, err
}

// ExportHostKeyRelativePath returns the relative path from spec/usage.yaml
// where the export host HMAC key is stored (issue 735).
func ExportHostKeyRelativePath() (string, error) {
	_, _, _, _, _, err := usageDurations()
	if err != nil {
		return "", err
	}
	if usageSpecHostKey == "" {
		return "export-host-key", nil
	}
	return usageSpecHostKey, nil
}

// PassiveDedupeInterval returns the collector's duplicate suppression interval.
func PassiveDedupeInterval() (time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecDedupe, err
}

func PassiveUpdatePath() (string, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecPassivePath, err
}

// UsageStoreBusyTimeout bounds best-effort writes during compact projection.
func UsageStoreBusyTimeout() (time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecStoreBusyTimeout, err
}

// ClientHeartbeatPolicy returns the client heartbeat interval, client expiry timeout,
// and catch-up fetch threshold from spec/usage.yaml (issue 714 M3).
func ClientHeartbeatPolicy() (time.Duration, time.Duration, time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecHeartbeatEvery, usageSpecClientTimeout, usageSpecCatchupThreshold, err
}

// RemoteCommandTimeouts returns the maximum remote command duration and SSH
// connection timeout from spec/usage.yaml.
func RemoteCommandTimeouts() (time.Duration, time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecRemoteCommandTimeout, usageSpecSSHConnectTimeout, err
}

// RemoteLoadPolicy returns the attached/detached polling intervals and how old
// a cached remote load reading may be before it is shown as stale (issues 879, 920).
func RemoteLoadPolicy() (time.Duration, time.Duration, time.Duration, error) {
	_, _, _, _, _, err := usageDurations()
	return usageSpecRemotePollAttached, usageSpecRemotePollDetached, usageSpecRemoteStaleness, err
}

func usageDurations() (time.Duration, time.Duration, time.Duration, time.Duration, time.Duration, error) {
	usageSpecOnce.Do(func() {
		data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
		if err != nil {
			usageSpecErr = fmt.Errorf("read embedded %s: %w", usageSpecPath, err)
			return
		}
		usageSpecAge, usageSpecCadence, usageSpecTimeout, usageSpecDedupe, usageSpecStoreBusyTimeout, usageSpecErr = parseUsageSpec(data)
		if usageSpecErr != nil {
			return
		}
		var spec usageSpec
		if err := yaml.Unmarshal(data, &spec); err == nil {
			usageSpecCachePolicy, _ = spec.cachePolicy() // Validated by parseUsageSpec.
			usageSpecPassivePath = spec.PassiveUpdatePath
			usageSpecLoadingLabel = spec.WatchLoadingLabel
			usageSpecHistoryEvery, _ = time.ParseDuration(spec.HistorySnapshotEvery)
			usageSpecHostKey = spec.ExportHostKey
			usageSpecHeartbeatEvery, _ = time.ParseDuration(spec.ClientHeartbeatInterval)
			usageSpecClientTimeout, _ = time.ParseDuration(spec.ClientTimeout)
			usageSpecCatchupThreshold, _ = time.ParseDuration(spec.CatchupFetchThreshold)
			usageSpecViewModes = spec.ViewModes
			usageSpecNVTimeout, _ = time.ParseDuration(spec.NVIDIASMITimeout)
			usageSpecNVCacheTTL, _ = time.ParseDuration(spec.NVIDIASMICacheTTL)
			usageSpecRemoteCommandTimeout, _ = time.ParseDuration(spec.RemoteCommandTimeout)
			usageSpecSSHConnectTimeout, _ = time.ParseDuration(spec.SSHConnectTimeout)
			usageSpecRemotePollAttached, _ = time.ParseDuration(spec.RemoteLoadPollAttached)
			usageSpecRemotePollDetached, _ = time.ParseDuration(spec.RemoteLoadPollDetached)
			usageSpecRemoteStaleness, _ = time.ParseDuration(spec.RemoteLoadStaleness)
		}
	})
	return usageSpecAge, usageSpecCadence, usageSpecTimeout, usageSpecDedupe, usageSpecStoreBusyTimeout, usageSpecErr
}

func parseUsageSpec(data []byte) (time.Duration, time.Duration, time.Duration, time.Duration, time.Duration, error) {
	var spec usageSpec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("parse embedded %s: %w", usageSpecPath, err)
	}
	if spec.WatchLoadingLabel == "" {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: watch_loading_label must not be empty", usageSpecPath)
	}
	if err := spec.ViewModes.validate(); err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: %w", usageSpecPath, err)
	}
	age, err := time.ParseDuration(spec.AGYMeterMaxAge)
	if err != nil || age <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: agy_meter_max_age must be a positive duration", usageSpecPath)
	}
	cachePolicy, err := spec.cachePolicy()
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	if _, _, err := spec.quotaNoticePolicy(); err != nil {
		return 0, 0, 0, 0, 0, err
	}
	timeout, err := time.ParseDuration(spec.CollectorTimeout)
	if err != nil || timeout <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: collector_timeout must be a positive duration", usageSpecPath)
	}
	dedupe, err := time.ParseDuration(spec.PassiveDedupeInterval)
	if err != nil || dedupe <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: passive_dedupe_interval must be a positive duration", usageSpecPath)
	}
	if !strings.HasPrefix(spec.PassiveUpdatePath, "/") || strings.ContainsAny(spec.PassiveUpdatePath, "?#") {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: passive_update_path must be an absolute route path", usageSpecPath)
	}
	if every, err := time.ParseDuration(spec.HistorySnapshotEvery); err != nil || every <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: history_snapshot_interval must be a positive duration", usageSpecPath)
	}
	if strings.TrimSpace(spec.ExportHostKey) == "" {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: export_host_key must not be empty", usageSpecPath)
	}
	if nvTimeout, err := time.ParseDuration(spec.NVIDIASMITimeout); err != nil || nvTimeout <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: nvidia_smi_timeout must be a positive duration", usageSpecPath)
	}
	if nvTTL, err := time.ParseDuration(spec.NVIDIASMICacheTTL); err != nil || nvTTL <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: nvidia_smi_cache_ttl must be a positive duration", usageSpecPath)
	}
	if remoteTimeout, err := time.ParseDuration(spec.RemoteCommandTimeout); err != nil || remoteTimeout <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: remote_command_timeout must be a positive duration", usageSpecPath)
	}
	if connectTimeout, err := time.ParseDuration(spec.SSHConnectTimeout); err != nil || connectTimeout <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: ssh_connect_timeout must be a positive duration", usageSpecPath)
	}
	remotePollAttached, err := time.ParseDuration(spec.RemoteLoadPollAttached)
	if err != nil || remotePollAttached <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: remote_load_poll_interval_attached must be a positive duration", usageSpecPath)
	}
	remotePollDetached, err := time.ParseDuration(spec.RemoteLoadPollDetached)
	if err != nil || remotePollDetached <= 0 || remotePollDetached < remotePollAttached {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: remote_load_poll_interval_detached must be positive and no shorter than remote_load_poll_interval_attached", usageSpecPath)
	}
	remoteStaleness, err := time.ParseDuration(spec.RemoteLoadStaleness)
	if err != nil || remoteStaleness <= remotePollAttached {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: remote_load_staleness must be longer than the attached poll interval", usageSpecPath)
	}
	if hbInterval, err := time.ParseDuration(spec.ClientHeartbeatInterval); err != nil || hbInterval <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: client_heartbeat_interval must be a positive duration", usageSpecPath)
	}
	if clientTimeout, err := time.ParseDuration(spec.ClientTimeout); err != nil || clientTimeout <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: client_timeout must be a positive duration", usageSpecPath)
	}
	if catchup, err := time.ParseDuration(spec.CatchupFetchThreshold); err != nil || catchup <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: catchup_fetch_threshold must be a positive duration", usageSpecPath)
	}
	storeBusy, err := time.ParseDuration(spec.UsageStoreBusyTimeout)
	if err != nil || storeBusy <= 0 || storeBusy > time.Second {
		return 0, 0, 0, 0, 0, fmt.Errorf("%s: usage_store_busy_timeout must be between 1ns and 1s", usageSpecPath)
	}
	return age, cachePolicy.cadence, timeout, dedupe, storeBusy, nil
}

func (s usageViewModesSpec) validate() error {
	if s.Default != "normal" && s.Default != "compact" && s.Default != "minimal" {
		return fmt.Errorf("view_modes.default must be normal, compact, or minimal")
	}
	if len(s.Modes) != 3 {
		return fmt.Errorf("view_modes.modes must define normal, compact, and minimal")
	}
	validPanels := map[string]bool{
		"table": true, "all_usage": true, "processes": true, "load": true,
		"mic": true, "remote_load": true,
	}
	for _, name := range []string{"normal", "compact", "minimal"} {
		mode, ok := s.Modes[name]
		if !ok || len(mode.Panels) == 0 {
			return fmt.Errorf("view_modes.modes.%s must define at least one panel", name)
		}
		if mode.TokenDetails == nil || mode.TitleBar == nil || mode.StatusBar == nil || mode.HiddenList == nil || mode.OverflowHint == nil {
			return fmt.Errorf("view_modes.modes.%s must define token_details, title_bar, status_bar, hidden_list, and overflow_hint", name)
		}
		seen := make(map[string]bool, len(mode.Panels))
		for _, panel := range mode.Panels {
			if !validPanels[panel] {
				return fmt.Errorf("view_modes.modes.%s contains unknown panel %q", name, panel)
			}
			if seen[panel] {
				return fmt.Errorf("view_modes.modes.%s repeats panel %q", name, panel)
			}
			seen[panel] = true
		}
	}
	return nil
}

func usageViewMode(mode string, compact, minimal bool) usageViewModeSpec {
	mode = usageViewModeName(mode, compact, minimal)
	definition, ok := usageSpecViewModes.Modes[mode]
	if !ok {
		panic(fmt.Sprintf("%s: unknown usage view mode %q", usageSpecPath, mode))
	}
	return definition
}

func usageViewModeName(mode string, compact, minimal bool) string {
	_, _, _, _, _, err := usageDurations()
	if err != nil {
		panic(err)
	}
	if mode == "" {
		switch {
		case minimal:
			mode = "minimal"
		case compact:
			mode = "compact"
		default:
			mode = usageSpecViewModes.Default
		}
	}
	if _, ok := usageSpecViewModes.Modes[mode]; !ok {
		panic(fmt.Sprintf("%s: unknown usage view mode %q", usageSpecPath, mode))
	}
	return mode
}

// DefaultUsageViewMode returns the mode declared by the embedded usage spec.
func DefaultUsageViewMode() (string, error) {
	_, _, _, _, _, err := usageDurations()
	if err != nil {
		return "", err
	}
	return usageSpecViewModes.Default, nil
}
