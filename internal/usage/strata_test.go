package usage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	strataLiveJSON = `{"live":{"tok_s":61.5},"requests":[{"decode_tok_s":52.0}]}`
	strataIdleJSON = `{"live":{"tok_s":null},"requests":[{"decode_tok_s":52.25}]}`
)

func strataServer(t *testing.T, metrics string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			_, _ = w.Write([]byte(metrics))
		case "/props":
			_, _ = w.Write([]byte(`{"model_alias":"qwen-coder"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchStrataLive(t *testing.T) {
	st := fetchStrata(t.Context(), strataServer(t, strataLiveJSON).URL, time.Second)
	if st == nil || st.Idle || st.TPS != 61.5 || st.Label != "qwen-coder" {
		t.Fatalf("got %+v, want live 61.5 qwen-coder", st)
	}
}

func TestFetchStrataIdleUsesLastRequest(t *testing.T) {
	st := fetchStrata(t.Context(), strataServer(t, strataIdleJSON).URL, time.Second)
	if st == nil || !st.Idle || st.TPS != 52.25 {
		t.Fatalf("got %+v, want idle 52.25", st)
	}
	line := stripANSI(buildStrataBox(40, *st).lines[0])
	if line != "qwen-coder 52.2 tok/s idle" && line != "qwen-coder 52.3 tok/s idle" {
		t.Errorf("box line = %q", line)
	}
}

func TestFetchStrataUnreachableAndMalformedHidden(t *testing.T) {
	srv := strataServer(t, `not json`)
	if st := fetchStrata(t.Context(), srv.URL, time.Second); st != nil {
		t.Errorf("malformed JSON: got %+v, want nil", st)
	}
	srv.Close()
	if st := fetchStrata(t.Context(), srv.URL, time.Second); st != nil {
		t.Errorf("unreachable: got %+v, want nil", st)
	}
	if parseStrataMetrics([]byte(`{}`)) != nil {
		t.Error("metrics without TPS must be hidden")
	}
}

func strataFrameText(t *testing.T, st *StrataStatus) string {
	t.Helper()
	old := strataStatusSource
	strataStatusSource = func() *StrataStatus { return st }
	t.Cleanup(func() { strataStatusSource = old })
	sec := initialWatchSections(WatchOptions{Mode: "minimal"})
	summary := UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Name: "Claude", Installed: true}}}
	frame := buildWatchFrame(summary, nil, time.Minute, sec, 140, 60, false)
	return stripANSI(strings.Join(frame.lines, "\n"))
}

func TestMinimalFrameShowsStrataBoxOnlyWhenOnline(t *testing.T) {
	on := strataFrameText(t, &StrataStatus{Label: "qwen-coder", TPS: 61.5})
	if !strings.Contains(on, "Strata") || !strings.Contains(on, "61.5 tok/s") {
		t.Errorf("online frame lacks Strata box:\n%s", on)
	}
	if off := strataFrameText(t, nil); strings.Contains(off, "Strata") {
		t.Errorf("offline frame must hide Strata box:\n%s", off)
	}
}

func TestCurrentStrataStatusCachesWithinTTL(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			hits++
			_, _ = w.Write([]byte(strataLiveJSON))
		}
	}))
	defer srv.Close()
	if _, _, _, _, _, err := usageDurations(); err != nil {
		t.Fatal(err)
	}
	old := usageSpecStrata
	usageSpecStrata = strataPolicy{endpoints: []string{srv.URL}, timeout: time.Second, ttl: time.Minute}
	strataCache.Lock()
	strataCache.at = time.Time{}
	strataCache.Unlock()
	t.Cleanup(func() {
		usageSpecStrata = old
		strataCache.Lock()
		strataCache.at = time.Time{}
		strataCache.Unlock()
	})
	for range 3 {
		if CurrentStrataStatus() == nil {
			t.Fatal("want status")
		}
	}
	if hits != 1 {
		t.Errorf("metrics hits = %d, want 1 (cached)", hits)
	}
}

func TestStrataSpecLoaded(t *testing.T) {
	if _, _, _, _, _, err := usageDurations(); err != nil {
		t.Fatal(err)
	}
	p := usageSpecStrata
	if len(p.endpoints) != 1 || p.endpoints[0] != "http://localhost:8080" || p.timeout != time.Second || p.ttl != 2*time.Second {
		t.Errorf("strata policy = %+v", p)
	}
	for _, m := range []string{"compact", "minimal"} {
		if !initialWatchSections(WatchOptions{Mode: m}).Strata {
			t.Errorf("%s mode lacks strata panel", m)
		}
	}
}

// Frame tests must never see a real local Strata server; tests that want a
// reading install their own source.
func init() { strataStatusSource = func() *StrataStatus { return nil } }

func TestParseStrataMetricsPrefillIsNotIdle(t *testing.T) {
	body := `{"live":{"state":"reading","tok_s":null,"prompt_read":4000,"prompt_total":60000,"prefill_tok_s_mean":421.2},"requests":[{"decode_tok_s":52.0}]}`
	st := parseStrataMetrics([]byte(body))
	if st == nil || !st.Prefill || st.Idle || st.PromptRead != 4000 || st.PromptTotal != 60000 || st.TPS != 421.2 {
		t.Fatalf("got %+v, want prefill 4000/60000 at 421.2", st)
	}
	line := buildStrataBox(60, *st).lines[0]
	if !strings.Contains(line, "reading prompt 4000/60000 421 tok/s") || strings.Contains(line, "idle") {
		t.Fatalf("box line %q", line)
	}
}
