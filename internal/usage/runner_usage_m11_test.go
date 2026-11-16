package usage

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/modelblock"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestRunnerUsageReadsRouteThroughBrokerWithoutSharedSQLite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	// Seed a real canonical SQLite database in an isolated host-only dir,
	// while placing corrupt bytes at runner's UsageStorePath to prove no direct DB access.
	hostDB := filepath.Join(t.TempDir(), "host-telemetry.sqlite")
	now := time.Now().UTC().Truncate(time.Second)
	reset := now.Add(time.Hour)
	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{{
			AgentID:       "codex",
			Name:          "OpenAI Codex",
			Installed:     true,
			Authenticated: true,
		}},
	}
	store, err := usagestore.Open(hostDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureUsageSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(summary)
	if err := store.WriteUsageSummary(context.Background(), usagestore.UsageSummaryRecord{
		Hostname:   "host-a",
		ObservedAt: now,
		Payload:    payload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteLatestUsageSummary(context.Background(), usagestore.UsageSummaryRecord{
		Hostname:   "host-a",
		ObservedAt: now,
		Payload:    payload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteCurrent(context.Background(), now, []usagestore.Window{{
		Provider: "codex", Pool: "primary", Key: "five_hour", Name: "five_hour",
		Source: "collector", Freshness: "fresh", UsedFraction: 0.4, ResetAt: &reset, ObservedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	// Write corrupt bytes where direct UsageStorePath would look.
	sharedDB := UsageStorePath()
	if err := os.MkdirAll(filepath.Dir(sharedDB), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedDB, []byte("poison-sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q server.RunnerUsageQuery
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch q.Action {
		case server.RunnerUsageActionLatestSummary:
			s, err := ReadStoredSummary(r.Context(), hostDB)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(s)
		case server.RunnerUsageActionProviderQuota:
			avail := ProviderQuotaAvailabilitiesFromStoreReadOnly(r.Context(), hostDB, q.Provider, q.Models, time.Now())
			_ = json.NewEncoder(w).Encode(avail)
		case server.RunnerUsageActionHistoryEntries:
			entries, err := ReadUsageHistoryFromStore(r.Context(), hostDB)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(entries)
		case server.RunnerUsageActionQuotaHistory:
			qEntries, err := ReadQuotaHistoryFromStore(r.Context(), hostDB)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(qEntries)
		default:
			http.Error(w, "forbidden", http.StatusForbidden)
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	hostEnd, runnerEnd, err := server.SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.RelayRunnerBroker(ctx, hostEnd, hostSocket, cfg) }()
	t.Cleanup(func() { cancel(); _ = runnerEnd.Close(); <-done })

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	// ReadStoredSummary routes through broker without touching sharedDB.
	gotSummary, err := ReadStoredSummary(context.Background(), "")
	if err != nil || len(gotSummary.Agents) != 1 || gotSummary.Agents[0].AgentID != "codex" {
		t.Fatalf("ReadStoredSummary(runner) = (%+v, %v)", gotSummary, err)
	}
	// Provider quota reads go through broker and merge local overlay modelblocks.
	if err := modelblock.Add(modelblock.Path(), modelblock.Block{
		Provider: "codex", Model: "blocked-model", Until: now.Add(time.Hour), Reason: "runner block", CreatedAt: now,
	}, now); err != nil {
		t.Fatal(err)
	}
	avail := CachedProviderQuotaAvailabilities("codex", []string{"gpt-5", "blocked-model"}, now)
	if avail["gpt-5"].State != "available" {
		t.Fatalf("broker quota avail(gpt-5) = %+v, want available", avail["gpt-5"])
	}
	if !avail["blocked-model"].Exhausted || avail["blocked-model"].BlockReason != "runner block" {
		t.Fatalf("overlay model block not applied: %+v", avail["blocked-model"])
	}
	if ro := CachedProviderQuotaAvailabilityReadOnly("codex", "gpt-5", now); ro.State != "available" {
		t.Fatalf("CachedProviderQuotaAvailabilityReadOnly(runner) = %+v, want available", ro)
	}

	// History & quota-history reads go through broker; stats skips local disk globs cleanly.
	histEntries, err := ReadHistory(HistoryDir(""))
	if err != nil || len(histEntries) != 1 || histEntries[0].Hostname != "host-a" {
		t.Fatalf("ReadHistory(runner) = (%+v, %v)", histEntries, err)
	}
	qHist, err := ReadQuotaHistory(HistoryDir(""))
	if err != nil || len(qHist) == 0 || qHist[0].Agent != "codex" {
		t.Fatalf("ReadQuotaHistory(runner) = (%+v, %v)", qHist, err)
	}
	stats, err := HistorySummaryStats(HistoryDir(""))
	if err != nil || stats.TotalEntries != 1 {
		t.Fatalf("HistorySummaryStats(runner) = (%+v, %v), want TotalEntries=1", stats, err)
	}

	// Canonical writes and compatibility imports remain host-only in runner mode.
	if err := AppendHistory(HistoryDir(""), summary); err == nil {
		t.Fatal("expected AppendHistory on canonical dir in runner mode to fail")
	}
	if err := ImportUsageCompatibility(context.Background(), "", ""); err == nil {
		t.Fatal("expected ImportUsageCompatibility in runner mode to fail")
	}

	// Removing broker FD fails closed on ReadStoredSummary & degrades quota to unknown.
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	if _, err := ReadStoredSummary(context.Background(), ""); err == nil {
		t.Fatal("expected ReadStoredSummary without broker FD to fail closed")
	}
	if ro := CachedProviderQuotaAvailabilityReadOnly("codex", "gpt-5", now); ro.State != "unknown" {
		t.Fatalf("without broker FD got %+v, want unknown", ro)
	}
}

func TestRunnerUsageDirectStoreHistoryGuards(t *testing.T) {
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	if _, err := UsageHistoryFromStore(context.Background(), "", ""); err == nil {
		t.Fatal("expected UsageHistoryFromStore to fail closed in runner mode")
	}
	if _, err := QuotaHistoryFromStore(context.Background(), "", ""); err == nil {
		t.Fatal("expected QuotaHistoryFromStore to fail closed in runner mode")
	}
}
