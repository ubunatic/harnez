package assess

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MultiDocSnapshot captures aggregate metrics at a specific point in time across all active documents.
type MultiDocSnapshot struct {
	CommitTime    time.Time      `json:"commit_time"`
	CommitSHA     string         `json:"commit_sha,omitempty"`
	CommitSubject string         `json:"commit_subject,omitempty"`
	TotalTokens   int            `json:"total_tokens"`
	TotalBytes    int64          `json:"total_bytes"`
	TotalLines    int            `json:"total_lines"`
	TotalWords    int            `json:"total_words"`
	ActiveDocs    int            `json:"active_docs"`
	ByDocTokens   map[string]int `json:"by_doc_tokens"`
	ByCategory    map[string]int `json:"by_category"` // e.g. "lang", "practices", "studies", "other", "root"
}

// MultiDocSummary summarizes metrics for a single document across the analyzed timeframe.
type MultiDocSummary struct {
	Path          string    `json:"path"`
	Category      string    `json:"category"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	StartTokens   int       `json:"start_tokens"`
	PeakTokens    int       `json:"peak_tokens"`
	CurrentTokens int       `json:"current_tokens"`
	CurrentBytes  int64     `json:"current_bytes"`
	CurrentLines  int       `json:"current_lines"`
	CurrentWords  int       `json:"current_words"`
	PctOfTotal    float64   `json:"pct_of_total"`
	Sparkline     string    `json:"sparkline"`
	TotalCommits  int       `json:"total_commits"`
}

// MultiDocResult holds the aggregate history, timeline snapshots, summaries, and telemetry.
type MultiDocResult struct {
	RepoDir       string             `json:"repo_dir"`
	Targets       []string           `json:"targets"`
	ResolvedFiles []string           `json:"resolved_files"`
	DocResults    []DocHistoryResult `json:"doc_results"`
	Timeline      []MultiDocSnapshot `json:"timeline"` // Chronological: oldest to newest
	DocSummaries  []MultiDocSummary  `json:"doc_summaries"`
	TotalCommits  int                `json:"total_commits"`
	UniqueBlobs   int                `json:"unique_blobs"`
	CacheHits     int                `json:"cache_hits"`
	CacheMisses   int                `json:"cache_misses"`
	Duration      time.Duration      `json:"duration"`
}

// CategorizeDoc assigns a human-friendly category string to a document path.
func CategorizeDoc(relPath string) string {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	parts := strings.Split(clean, "/")
	if len(parts) == 1 {
		return "root"
	}
	if parts[0] == "docs" && len(parts) > 2 {
		return parts[1] // e.g., docs/lang/Go.md -> "lang", docs/studies/... -> "studies"
	}
	if parts[0] == "docs" {
		return "docs"
	}
	return parts[0]
}

// DiscoverDocFiles expands directories, globs, or explicit file paths into a sorted list of unique repository-relative files.
func DiscoverDocFiles(repoDir string, targets []string) ([]string, error) {
	if len(targets) == 0 {
		targets = []string{"docs", "AGENTS.md"}
	}

	seen := make(map[string]bool)
	var files []string

	for _, target := range targets {
		// 1. Check if target contains glob patterns
		if strings.ContainsAny(target, "*?[]") {
			fullPattern := target
			if repoDir != "" && !filepath.IsAbs(target) {
				fullPattern = filepath.Join(repoDir, target)
			}
			matches, err := filepath.Glob(fullPattern)
			if err != nil {
				return nil, fmt.Errorf("glob pattern %q failed: %w", target, err)
			}
			for _, m := range matches {
				rel := m
				if repoDir != "" {
					rel, _ = filepath.Rel(repoDir, m)
				}
				rel = filepath.ToSlash(rel)
				fi, err := os.Stat(m)
				if err == nil && !fi.IsDir() {
					if !seen[rel] {
						seen[rel] = true
						files = append(files, rel)
					}
				}
			}
			continue
		}

		fullPath := target
		if repoDir != "" && !filepath.IsAbs(target) {
			fullPath = filepath.Join(repoDir, target)
		}

		fi, err := os.Stat(fullPath)
		if err != nil {
			// Might be a file tracked in git or non-existent working copy file; keep rel path if not dir
			rel := filepath.ToSlash(target)
			if !seen[rel] {
				seen[rel] = true
				files = append(files, rel)
			}
			continue
		}

		if fi.IsDir() {
			err := filepath.WalkDir(fullPath, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				// Include markdown, text, or documentation files
				ext := strings.ToLower(filepath.Ext(p))
				if ext == ".md" || ext == ".txt" || ext == ".rst" || filepath.Base(p) == "AGENTS.md" || filepath.Base(p) == "README" {
					rel := p
					if repoDir != "" {
						rel, _ = filepath.Rel(repoDir, p)
					}
					rel = filepath.ToSlash(rel)
					if !seen[rel] {
						seen[rel] = true
						files = append(files, rel)
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk dir %q: %w", fullPath, err)
			}
		} else {
			rel := target
			if repoDir != "" && filepath.IsAbs(target) {
				rel, _ = filepath.Rel(repoDir, target)
			}
			rel = filepath.ToSlash(rel)
			if !seen[rel] {
				seen[rel] = true
				files = append(files, rel)
			}
		}
	}

	sort.Strings(files)
	return files, nil
}

// ExtractMultiDocHistory extracts git histories for all given targets concurrently and builds a unified timeline.
func ExtractMultiDocHistory(repoDir string, targets []string) (*MultiDocResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ExtractMultiDocHistoryContext(ctx, repoDir, targets)
}

// ExtractMultiDocHistoryContext extracts multi-document history with context support.
func ExtractMultiDocHistoryContext(ctx context.Context, repoDir string, targets []string) (*MultiDocResult, error) {
	start := time.Now()

	resolvedFiles, err := DiscoverDocFiles(repoDir, targets)
	if err != nil {
		return nil, fmt.Errorf("discover doc files: %w", err)
	}

	if len(resolvedFiles) == 0 {
		return &MultiDocResult{
			RepoDir:       repoDir,
			Targets:       targets,
			ResolvedFiles: nil,
			Duration:      time.Since(start),
		}, nil
	}

	// Concurrently extract history for each file
	type fileRes struct {
		idx int
		res *DocHistoryResult
		err error
	}

	resChan := make(chan fileRes, len(resolvedFiles))
	var wg sync.WaitGroup

	// Limit concurrency to avoid spawning too many git processes simultaneously
	concurrency := 8
	sem := make(chan struct{}, concurrency)

	for i, file := range resolvedFiles {
		wg.Add(1)
		go func(idx int, fPath string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				resChan <- fileRes{idx: idx, err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			dRes, dErr := ExtractDocHistoryContext(ctx, repoDir, fPath)
			resChan <- fileRes{idx: idx, res: dRes, err: dErr}
		}(i, file)
	}

	wg.Wait()
	close(resChan)

	docResults := make([]DocHistoryResult, len(resolvedFiles))
	totalCacheHits := 0
	totalCacheMisses := 0
	totalBlobsSet := make(map[string]bool)
	allCommitsSet := make(map[string]bool)

	for r := range resChan {
		if r.err != nil {
			// If file not tracked or failed, keep empty result
			continue
		}
		if r.res != nil {
			docResults[r.idx] = *r.res
			totalCacheHits += r.res.CacheHits
			totalCacheMisses += r.res.CacheMisses
			for _, snap := range r.res.Snapshots {
				allCommitsSet[snap.CommitSHA] = true
				if snap.BlobSHA != "" && snap.BlobSHA != "0000000000000000000000000000000000000000" {
					totalBlobsSet[snap.BlobSHA] = true
				}
			}
		}
	}

	// Build unified timeline & summaries
	timeline, summaries := BuildUnifiedTimeline(resolvedFiles, docResults)

	return &MultiDocResult{
		RepoDir:       repoDir,
		Targets:       targets,
		ResolvedFiles: resolvedFiles,
		DocResults:    docResults,
		Timeline:      timeline,
		DocSummaries:  summaries,
		TotalCommits:  len(allCommitsSet),
		UniqueBlobs:   len(totalBlobsSet),
		CacheHits:     totalCacheHits,
		CacheMisses:   totalCacheMisses,
		Duration:      time.Since(start),
	}, nil
}

// docEvent represents an update to a specific document at a specific timestamp.
type docEvent struct {
	time          time.Time
	commitSHA     string
	commitSubject string
	docIdx        int
	snapshot      DocSnapshot
}

// BuildUnifiedTimeline aligns multiple document histories into a single chronological timeline.
func BuildUnifiedTimeline(resolvedFiles []string, docResults []DocHistoryResult) ([]MultiDocSnapshot, []MultiDocSummary) {
	// Collect all events across all documents
	var events []docEvent
	for docIdx, dRes := range docResults {
		for _, snap := range dRes.Snapshots {
			events = append(events, docEvent{
				time:          snap.CommitTime,
				commitSHA:     snap.CommitSHA,
				commitSubject: snap.CommitSubject,
				docIdx:        docIdx,
				snapshot:      snap,
			})
		}
	}

	if len(events) == 0 {
		return nil, nil
	}

	// Sort events chronologically (oldest to newest)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].time.Equal(events[j].time) {
			return events[i].commitSHA < events[j].commitSHA
		}
		return events[i].time.Before(events[j].time)
	})

	// Track active state per document across time
	type liveState struct {
		active     bool
		metrics    DocMetrics
		tokensHist []int
		firstSeen  time.Time
		lastSeen   time.Time
		peakTokens int
	}

	docStates := make([]liveState, len(resolvedFiles))
	for i := range docStates {
		docStates[i] = liveState{
			active:     false,
			tokensHist: make([]int, 0),
		}
	}

	var timeline []MultiDocSnapshot

	// Process events in chronological sequence
	for _, ev := range events {
		st := &docStates[ev.docIdx]

		isDeleted := ev.snapshot.BlobSHA == "" || ev.snapshot.BlobSHA == "0000000000000000000000000000000000000000" || ev.snapshot.Status == "D"

		if isDeleted {
			st.active = false
			st.metrics = DocMetrics{}
			st.lastSeen = ev.time
		} else {
			if !st.active && st.firstSeen.IsZero() {
				st.firstSeen = ev.time
			}
			st.active = true
			st.lastSeen = ev.time
			st.metrics = DocMetrics{
				RawBytes:   ev.snapshot.RawBytes,
				Lines:      ev.snapshot.Lines,
				Words:      ev.snapshot.Words,
				EstTokens:  ev.snapshot.EstTokens,
				Headings:   ev.snapshot.Headings,
				CodeBlocks: ev.snapshot.CodeBlocks,
			}
			if ev.snapshot.EstTokens > st.peakTokens {
				st.peakTokens = ev.snapshot.EstTokens
			}
		}
		st.tokensHist = append(st.tokensHist, st.metrics.EstTokens)

		// Aggregate current snapshot across all active documents
		totalTok := 0
		var totalBytes int64
		totalLines := 0
		totalWords := 0
		activeDocs := 0
		byDoc := make(map[string]int)
		byCat := make(map[string]int)

		for dIdx, ds := range docStates {
			if ds.active {
				activeDocs++
				path := resolvedFiles[dIdx]
				cat := CategorizeDoc(path)
				tok := ds.metrics.EstTokens

				totalTok += tok
				totalBytes += ds.metrics.RawBytes
				totalLines += ds.metrics.Lines
				totalWords += ds.metrics.Words

				byDoc[path] = tok
				byCat[cat] += tok
			}
		}

		snap := MultiDocSnapshot{
			CommitTime:    ev.time,
			CommitSHA:     ev.commitSHA,
			CommitSubject: ev.commitSubject,
			TotalTokens:   totalTok,
			TotalBytes:    totalBytes,
			TotalLines:    totalLines,
			TotalWords:    totalWords,
			ActiveDocs:    activeDocs,
			ByDocTokens:   byDoc,
			ByCategory:    byCat,
		}
		timeline = append(timeline, snap)
	}

	// Compute summaries for each document
	finalTotalTokens := 0
	if len(timeline) > 0 {
		finalTotalTokens = timeline[len(timeline)-1].TotalTokens
	}

	summaries := make([]MultiDocSummary, 0, len(resolvedFiles))
	for dIdx, path := range resolvedFiles {
		st := docStates[dIdx]
		startTokens := 0
		if len(st.tokensHist) > 0 {
			startTokens = st.tokensHist[0]
		}
		currentTokens := st.metrics.EstTokens
		pct := 0.0
		if finalTotalTokens > 0 && st.active {
			pct = (float64(currentTokens) / float64(finalTotalTokens)) * 100.0
		}

		spark := RenderSparkline(st.tokensHist, 16)
		if len(st.tokensHist) == 0 {
			spark = strings.Repeat(" ", 16)
		}

		summaries = append(summaries, MultiDocSummary{
			Path:          path,
			Category:      CategorizeDoc(path),
			FirstSeen:     st.firstSeen,
			LastSeen:      st.lastSeen,
			StartTokens:   startTokens,
			PeakTokens:    st.peakTokens,
			CurrentTokens: currentTokens,
			CurrentBytes:  st.metrics.RawBytes,
			CurrentLines:  st.metrics.Lines,
			CurrentWords:  st.metrics.Words,
			PctOfTotal:    pct,
			Sparkline:     spark,
			TotalCommits:  len(st.tokensHist),
		})
	}

	// Sort summaries by CurrentTokens descending
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].CurrentTokens == summaries[j].CurrentTokens {
			return summaries[i].Path < summaries[j].Path
		}
		return summaries[i].CurrentTokens > summaries[j].CurrentTokens
	})

	return timeline, summaries
}

// MilestoneSnapshot represents sampled aggregate metrics at a specific key time milestone.
type MilestoneSnapshot struct {
	Label       string             `json:"label"`
	Time        time.Time          `json:"time"`
	TotalTokens int                `json:"total_tokens"`
	ActiveDocs  int                `json:"active_docs"`
	CategoryPct map[string]float64 `json:"category_pct"`
	CategoryTok map[string]int     `json:"category_tok"`
}

// SampleMilestones groups timeline entries into periodic milestones (e.g. monthly or evenly spaced + present).
func SampleMilestones(timeline []MultiDocSnapshot, maxMilestones int) []MilestoneSnapshot {
	if len(timeline) == 0 {
		return nil
	}

	if maxMilestones <= 0 {
		maxMilestones = 5
	}

	// Group snapshots by Month "YYYY-MM"
	type monthGroup struct {
		month string
		last  MultiDocSnapshot
	}
	var monthly []monthGroup
	monthSeen := make(map[string]int) // month -> index in monthly

	for _, snap := range timeline {
		mStr := snap.CommitTime.Format("2006-01")
		if idx, exists := monthSeen[mStr]; exists {
			monthly[idx].last = snap
		} else {
			monthSeen[mStr] = len(monthly)
			monthly = append(monthly, monthGroup{month: mStr, last: snap})
		}
	}

	var sampled []MultiDocSnapshot
	if len(monthly) <= maxMilestones {
		for _, mg := range monthly {
			sampled = append(sampled, mg.last)
		}
	} else {
		// Sample evenly across months
		step := float64(len(monthly)-1) / float64(maxMilestones-1)
		for i := 0; i < maxMilestones; i++ {
			idx := int(float64(i)*step + 0.5)
			if idx >= len(monthly) {
				idx = len(monthly) - 1
			}
			sampled = append(sampled, monthly[idx].last)
		}
	}

	// Ensure the latest snapshot (Present) is always included
	lastSnap := timeline[len(timeline)-1]
	if len(sampled) > 0 && !sampled[len(sampled)-1].CommitTime.Equal(lastSnap.CommitTime) {
		sampled[len(sampled)-1] = lastSnap
	}

	// Deduplicate if any consecutive have identical timestamps
	var deduped []MultiDocSnapshot
	for _, s := range sampled {
		if len(deduped) == 0 || !deduped[len(deduped)-1].CommitTime.Equal(s.CommitTime) {
			deduped = append(deduped, s)
		}
	}

	var res []MilestoneSnapshot
	for i, s := range deduped {
		label := s.CommitTime.Format("Jan 2006")
		if i == len(deduped)-1 {
			label = fmt.Sprintf("Present (%s)", s.CommitTime.Format("02 Jan"))
		}

		catPct := make(map[string]float64)
		catTok := make(map[string]int)
		for cat, tok := range s.ByCategory {
			catTok[cat] = tok
			if s.TotalTokens > 0 {
				catPct[cat] = (float64(tok) / float64(s.TotalTokens)) * 100.0
			}
		}

		res = append(res, MilestoneSnapshot{
			Label:       label,
			Time:        s.CommitTime,
			TotalTokens: s.TotalTokens,
			ActiveDocs:  s.ActiveDocs,
			CategoryPct: catPct,
			CategoryTok: catTok,
		})
	}

	return res
}

// CategoryStyle defines visually distinct block characters and ANSI colors for categories.
type CategoryStyle struct {
	Key       string
	Label     string
	Block     string
	ANSIColor string
}

// CategoryPalette assigns visually distinct colors and glyphs for categories.
var CategoryPalette = []CategoryStyle{
	{Key: "lang", Label: "lang", Block: "█", ANSIColor: "36"},             // Cyan
	{Key: "practices", Label: "practices", Block: "▓", ANSIColor: "32"},   // Green
	{Key: "studies", Label: "studies", Block: "▒", ANSIColor: "35"},       // Magenta
	{Key: "feedback", Label: "feedback", Block: "░", ANSIColor: "33"},     // Yellow
	{Key: "issues", Label: "issues", Block: "◈", ANSIColor: "38;5;208"},   // Orange
	{Key: "root", Label: "root (AGENTS.md)", Block: "■", ANSIColor: "34"}, // Blue
	{Key: "docs", Label: "docs", Block: "◆", ANSIColor: "37"},             // White
	{Key: "other", Label: "other", Block: "▲", ANSIColor: "31"},           // Red
}

// RenderStackedBar creates a stacked bar representing category token proportions.
func RenderStackedBar(categoryTok map[string]int, totalTokens int, width int, useColor bool) string {
	if totalTokens <= 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}

	type catShare struct {
		key   string
		glyph string
		color string
		tok   int
		chars int
	}

	var shares []catShare
	for _, p := range CategoryPalette {
		if tok, ok := categoryTok[p.Key]; ok && tok > 0 {
			shares = append(shares, catShare{
				key:   p.Key,
				glyph: p.Block,
				color: p.ANSIColor,
				tok:   tok,
			})
		}
	}
	// Also check any extra categories not in predefined palette
	for cat, tok := range categoryTok {
		found := false
		for _, s := range shares {
			if s.key == cat {
				found = true
				break
			}
		}
		if !found && tok > 0 {
			shares = append(shares, catShare{
				key:   cat,
				glyph: "●",
				color: "90",
				tok:   tok,
			})
		}
	}

	// Sort by token size descending
	sort.Slice(shares, func(i, j int) bool {
		return shares[i].tok > shares[j].tok
	})

	allocated := 0
	for i := range shares {
		c := int(float64(shares[i].tok)/float64(totalTokens)*float64(width) + 0.5)
		if c < 1 && shares[i].tok > 0 {
			c = 1
		}
		shares[i].chars = c
		allocated += c
	}

	// Adjust for rounding diffs
	for allocated > width {
		for i := len(shares) - 1; i >= 0 && allocated > width; i-- {
			if shares[i].chars > 1 {
				shares[i].chars--
				allocated--
			}
		}
		if allocated > width {
			shares[0].chars--
			allocated--
		}
	}
	for allocated < width && len(shares) > 0 {
		shares[0].chars++
		allocated++
	}

	var b strings.Builder
	for _, s := range shares {
		if s.chars <= 0 {
			continue
		}
		if useColor {
			b.WriteString(fmt.Sprintf("\x1b[%sm%s\x1b[0m", s.color, strings.Repeat("█", s.chars)))
		} else {
			b.WriteString(strings.Repeat(s.glyph, s.chars))
		}
	}
	return b.String()
}

// RenderMultiDocOptions configures the formatting and colorization of multi-doc reports.
type RenderMultiDocOptions struct {
	Color bool
}

// RenderMultiDocHistory renders the multi-document report to a terminal-formatted string.
func RenderMultiDocHistory(res *MultiDocResult, opts ...RenderMultiDocOptions) string {
	if res == nil || len(res.Timeline) == 0 {
		return "No document git history found for specified targets.\n"
	}

	useColor := false
	if len(opts) > 0 {
		useColor = opts[0].Color
	}

	var b strings.Builder
	b.WriteString("── Multi-Doc Token Evolution & Stacked Category History ────────────\n")

	// 1. Header Metrics
	hitRate := 0.0
	totalAccesses := res.CacheHits + res.CacheMisses
	if totalAccesses > 0 {
		hitRate = float64(res.CacheHits) / float64(totalAccesses) * 100.0
	}
	b.WriteString(fmt.Sprintf("Documents: %d analyzed | Commits: %d | Unique Blobs: %d | Cache Hits: %d (%.1f%%) | Latency: %v\n",
		len(res.ResolvedFiles), res.TotalCommits, res.UniqueBlobs, res.CacheHits, hitRate, res.Duration.Round(time.Millisecond)))

	// 2. Aggregate Sparkline & Overview
	tokensSeries := make([]int, len(res.Timeline))
	peakTok := 0
	for i, s := range res.Timeline {
		tokensSeries[i] = s.TotalTokens
		if s.TotalTokens > peakTok {
			peakTok = s.TotalTokens
		}
	}
	startTok := tokensSeries[0]
	endTok := tokensSeries[len(tokensSeries)-1]
	diffTok := endTok - startTok
	pctChange := 0.0
	if startTok > 0 {
		pctChange = float64(diffTok) / float64(startTok) * 100.0
	}
	sign := "+"
	if diffTok < 0 {
		sign = ""
	}
	sparkline := RenderSparkline(tokensSeries, 40)
	b.WriteString(fmt.Sprintf("Aggregate Sparkline: [%s]  (%s → %s tokens, %s%.1f%% | Peak: %s)\n\n",
		sparkline, formatNumber(startTok), formatNumber(endTok), sign, pctChange, formatNumber(peakTok)))

	// 3. Category Proportions Over Milestones (Stacked Breakdown)
	milestones := SampleMilestones(res.Timeline, 5)
	if len(milestones) > 0 {
		b.WriteString("── Category Breakdown Across Milestones ────────────────────────────\n")
		// Legend: only show categories that are actively present in the dataset
		activeCats := make(map[string]bool)
		for _, m := range milestones {
			for cat, tok := range m.CategoryTok {
				if tok > 0 {
					activeCats[cat] = true
				}
			}
		}
		for _, s := range res.DocSummaries {
			if s.CurrentTokens > 0 || s.PeakTokens > 0 {
				activeCats[s.Category] = true
			}
		}

		var legendParts []string
		for _, p := range CategoryPalette {
			if activeCats[p.Key] {
				if useColor {
					legendParts = append(legendParts, fmt.Sprintf("\x1b[%sm█\x1b[0m %s", p.ANSIColor, p.Label))
				} else {
					legendParts = append(legendParts, fmt.Sprintf("%s %s", p.Block, p.Label))
				}
				delete(activeCats, p.Key)
			}
		}
		for cat := range activeCats {
			if useColor {
				legendParts = append(legendParts, fmt.Sprintf("\x1b[90m█\x1b[0m %s", cat))
			} else {
				legendParts = append(legendParts, fmt.Sprintf("● %s", cat))
			}
		}

		b.WriteString(fmt.Sprintf("Legend: %s\n", strings.Join(legendParts, "  ")))
		b.WriteString(fmt.Sprintf("%-20s  %-30s  %10s  %6s\n", "MILESTONE", "PROPORTION BAR", "TOKENS", "DOCS"))
		b.WriteString(strings.Repeat("─", 74) + "\n")

		for _, m := range milestones {
			bar := RenderStackedBar(m.CategoryTok, m.TotalTokens, 30, useColor)
			b.WriteString(fmt.Sprintf("%-20s  [%s]  %10s  %6d\n",
				m.Label, bar, formatNumber(m.TotalTokens), m.ActiveDocs))
		}
		b.WriteString("\n")
	}

	// 4. Per-Document Summary Table
	b.WriteString("── Document Summary (Sorted by Current Token Footprint) ────────────\n")
	b.WriteString(fmt.Sprintf("%-36s  %10s  %8s  %8s  %8s  %6s  %s\n",
		"DOCUMENT", "CATEGORY", "START", "PEAK", "CURRENT", "% TOT", "TREND"))
	b.WriteString(strings.Repeat("─", 94) + "\n")

	for _, s := range res.DocSummaries {
		dispPath := s.Path
		if len(dispPath) > 36 {
			dispPath = "..." + dispPath[len(dispPath)-33:]
		}

		b.WriteString(fmt.Sprintf("%-36s  %-10s  %8s  %8s  %8s  %5.1f%%  [%s]\n",
			dispPath,
			s.Category,
			formatNumber(s.StartTokens),
			formatNumber(s.PeakTokens),
			formatNumber(s.CurrentTokens),
			s.PctOfTotal,
			s.Sparkline,
		))
	}

	return b.String()
}
