package usage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// strataPolicy is the spec-driven Strata TPS box configuration (issue 1182).
type strataPolicy struct {
	endpoints []string
	timeout   time.Duration
	ttl       time.Duration
}

// StrataStatus is one Strata server reading: the current decode speed while a
// request is generating, else the last finished request's speed (Idle).
type StrataStatus struct {
	Label string
	TPS   float64
	Idle  bool
	// Prefill is true while a request reads its prompt (e.g. compaction): no
	// decode speed exists yet, so TPS holds the prefill speed when known.
	Prefill bool
	// PromptRead and PromptTotal are the prompt progress when the server reports it.
	PromptRead, PromptTotal int
}

type strataMetrics struct {
	Live struct {
		State           string   `json:"state"`
		TokS            *float64 `json:"tok_s"`
		PrefillTokS     *float64 `json:"prefill_tok_s"`
		PrefillTokSMean *float64 `json:"prefill_tok_s_mean"`
		PromptRead      *int     `json:"prompt_read"`
		PromptTotal     *int     `json:"prompt_total"`
	} `json:"live"`
	Requests []struct {
		DecodeTokS *float64 `json:"decode_tok_s"`
	} `json:"requests"`
}

// parseStrataMetrics decodes /metrics JSON. It returns nil when the payload is
// malformed or carries no TPS value at all.
func parseStrataMetrics(body []byte) *StrataStatus {
	var m strataMetrics
	if json.Unmarshal(body, &m) != nil {
		return nil
	}
	if m.Live.TokS != nil {
		return &StrataStatus{TPS: *m.Live.TokS}
	}
	if st := m.Live.State; st != "" && st != "idle" {
		// Busy without a decode speed yet: prompt prefill (compaction, long input).
		out := &StrataStatus{Prefill: true}
		if m.Live.PrefillTokS != nil {
			out.TPS = *m.Live.PrefillTokS
		} else if m.Live.PrefillTokSMean != nil {
			out.TPS = *m.Live.PrefillTokSMean
		}
		if m.Live.PromptRead != nil && m.Live.PromptTotal != nil {
			out.PromptRead, out.PromptTotal = *m.Live.PromptRead, *m.Live.PromptTotal
		}
		return out
	}
	if len(m.Requests) > 0 && m.Requests[0].DecodeTokS != nil {
		return &StrataStatus{TPS: *m.Requests[0].DecodeTokS, Idle: true}
	}
	return nil
}

func strataGet(ctx context.Context, url string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, err == nil
}

// fetchStrata reads one endpoint; nil means offline or unusable.
func fetchStrata(ctx context.Context, base string, timeout time.Duration) *StrataStatus {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	base = strings.TrimRight(base, "/")
	body, ok := strataGet(ctx, base+"/metrics")
	if !ok {
		return nil
	}
	st := parseStrataMetrics(body)
	if st == nil {
		return nil
	}
	st.Label = "Strata"
	if props, ok := strataGet(ctx, base+"/props"); ok {
		var p struct {
			Alias string `json:"model_alias"`
		}
		if json.Unmarshal(props, &p) == nil && p.Alias != "" {
			st.Label = p.Alias
		}
	}
	return st
}

var strataCache struct {
	sync.Mutex
	at     time.Time
	status *StrataStatus
}

// CurrentStrataStatus returns the first answering endpoint's reading, or nil
// when none answers. Hits and misses are cached for the spec TTL so the 100ms
// redraw never probes an offline server each frame.
func CurrentStrataStatus() *StrataStatus {
	if _, _, _, _, _, err := usageDurations(); err != nil {
		return nil
	}
	pol := usageSpecStrata
	strataCache.Lock()
	defer strataCache.Unlock()
	if !strataCache.at.IsZero() && time.Since(strataCache.at) < pol.ttl {
		return strataCache.status
	}
	var got *StrataStatus
	for _, ep := range pol.endpoints {
		if got = fetchStrata(context.Background(), ep, pol.timeout); got != nil {
			break
		}
	}
	strataCache.at, strataCache.status = time.Now(), got
	return got
}

// strataStatusSource is a seam so frame tests need no network.
var strataStatusSource = CurrentStrataStatus
