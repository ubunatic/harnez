package assess

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DocSnapshot represents the document state at a specific git commit.
type DocSnapshot struct {
	CommitSHA     string    `json:"commit_sha"`
	CommitTime    time.Time `json:"commit_time"`
	CommitSubject string    `json:"commit_subject"`
	Path          string    `json:"path"`
	OldPath       string    `json:"old_path,omitempty"`
	Status        string    `json:"status"` // M, A, R100, etc.
	BlobSHA       string    `json:"blob_sha"`
	RawBytes      int64     `json:"raw_bytes"`
	Lines         int       `json:"lines"`
	Words         int       `json:"words"`
	EstTokens     int       `json:"est_tokens"`
	Headings      int       `json:"headings"`
	CodeBlocks    int       `json:"code_blocks"`
}

// DocMetrics holds computed metrics for a document's content.
type DocMetrics struct {
	RawBytes   int64 `json:"raw_bytes"`
	Lines      int   `json:"lines"`
	Words      int   `json:"words"`
	EstTokens  int   `json:"est_tokens"`
	Headings   int   `json:"headings"`
	CodeBlocks int   `json:"code_blocks"`
}

// DocHistoryResult contains the full time-series history and cache metrics.
type DocHistoryResult struct {
	RepoDir      string        `json:"repo_dir"`
	FilePath     string        `json:"file_path"`
	Snapshots    []DocSnapshot `json:"snapshots"` // Chronological: oldest to newest
	TotalCommits int           `json:"total_commits"`
	UniqueBlobs  int           `json:"unique_blobs"`
	CacheHits    int           `json:"cache_hits"`
	CacheMisses  int           `json:"cache_misses"`
	Duration     time.Duration `json:"duration"`
}

// CountWords counts whitespace-separated words in data.
func CountWords(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(bufio.ScanWords)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}

// CountHeadings counts Markdown heading lines (lines starting with '#').
func CountHeadings(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	count := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte("#")) {
			count++
		}
	}
	return count
}

// CountCodeBlocks counts fenced code blocks (lines starting with '```' or '~~~').
func CountCodeBlocks(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	fences := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte("```")) || bytes.HasPrefix(line, []byte("~~~")) {
			fences++
		}
	}
	return fences / 2
}

// ComputeDocMetrics computes metrics from raw byte contents.
func ComputeDocMetrics(content []byte) DocMetrics {
	return DocMetrics{
		RawBytes:   int64(len(content)),
		Lines:      CountLines(content),
		Words:      CountWords(content),
		EstTokens:  EstimateTokens(content),
		Headings:   CountHeadings(content),
		CodeBlocks: CountCodeBlocks(content),
	}
}

// rawCommitEntry stores intermediate parsed log entries.
type rawCommitEntry struct {
	commitSHA string
	unixTime  int64
	subject   string
	oldBlob   string
	newBlob   string
	status    string
	oldPath   string
	newPath   string
}

// ExtractDocHistory traverses git history for filePath in repoDir using
// 'git log --follow' and streaming blob extraction via 'git cat-file --batch'.
func ExtractDocHistory(repoDir, filePath string) (*DocHistoryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return ExtractDocHistoryContext(ctx, repoDir, filePath)
}

// ExtractDocHistoryContext traverses git history with context cancellation.
func ExtractDocHistoryContext(ctx context.Context, repoDir, filePath string) (*DocHistoryResult, error) {
	start := time.Now()

	// 1. Run git log --follow to extract all commits and blob transitions
	cmd := exec.CommandContext(ctx, "git", "log", "--follow", "--raw", "--abbrev=40", "--format=commit %H %at %s", "--", filePath)
	if repoDir != "" {
		cmd.Dir = repoDir
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	rawEntries, err := parseGitLogRaw(out)
	if err != nil {
		return nil, fmt.Errorf("parse git log: %w", err)
	}

	if len(rawEntries) == 0 {
		return &DocHistoryResult{
			RepoDir:      repoDir,
			FilePath:     filePath,
			Snapshots:    nil,
			TotalCommits: 0,
			Duration:     time.Since(start),
		}, nil
	}

	// 2. Collect unique blob SHAs to fetch via cat-file
	blobCache := make(map[string]DocMetrics)
	var blobsToFetch []string
	seenBlobs := make(map[string]bool)

	for _, entry := range rawEntries {
		if entry.newBlob != "" && entry.newBlob != "0000000000000000000000000000000000000000" {
			if !seenBlobs[entry.newBlob] {
				seenBlobs[entry.newBlob] = true
				blobsToFetch = append(blobsToFetch, entry.newBlob)
			}
		}
	}

	// 3. Batch fetch blobs using 'git cat-file --batch'
	fetchedMetrics, err := batchFetchBlobs(ctx, repoDir, blobsToFetch)
	if err != nil {
		return nil, fmt.Errorf("batch fetch blobs: %w", err)
	}

	for sha, metrics := range fetchedMetrics {
		blobCache[sha] = metrics
	}

	// 4. Build snapshots in chronological order (oldest to newest)
	// git log returns newest to oldest, so reverse
	totalCommits := len(rawEntries)
	snapshots := make([]DocSnapshot, 0, totalCommits)
	cacheHits := 0
	cacheMisses := 0

	accessedBlobs := make(map[string]bool)

	for i := len(rawEntries) - 1; i >= 0; i-- {
		entry := rawEntries[i]
		blob := entry.newBlob

		var m DocMetrics
		if blob == "" || blob == "0000000000000000000000000000000000000000" {
			// Deleted
			m = DocMetrics{}
		} else {
			if accessedBlobs[blob] {
				cacheHits++
			} else {
				cacheMisses++
				accessedBlobs[blob] = true
			}
			m = blobCache[blob]
		}

		snap := DocSnapshot{
			CommitSHA:     entry.commitSHA,
			CommitTime:    time.Unix(entry.unixTime, 0).UTC(),
			CommitSubject: entry.subject,
			Path:          entry.newPath,
			OldPath:       entry.oldPath,
			Status:        entry.status,
			BlobSHA:       blob,
			RawBytes:      m.RawBytes,
			Lines:         m.Lines,
			Words:         m.Words,
			EstTokens:     m.EstTokens,
			Headings:      m.Headings,
			CodeBlocks:    m.CodeBlocks,
		}
		snapshots = append(snapshots, snap)
	}

	return &DocHistoryResult{
		RepoDir:      repoDir,
		FilePath:     filePath,
		Snapshots:    snapshots,
		TotalCommits: totalCommits,
		UniqueBlobs:  len(blobsToFetch),
		CacheHits:    cacheHits,
		CacheMisses:  cacheMisses,
		Duration:     time.Since(start),
	}, nil
}

// parseGitLogRaw parses output of 'git log --raw --abbrev=40 --format="commit %H %at %s"'.
func parseGitLogRaw(output []byte) ([]rawCommitEntry, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var entries []rawCommitEntry
	var currentEntry *rawCommitEntry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "commit ") {
			if currentEntry != nil {
				entries = append(entries, *currentEntry)
				currentEntry = nil
			}

			// Format: commit <SHA> <timestamp> <subject>
			parts := strings.SplitN(line[len("commit "):], " ", 3)
			if len(parts) < 2 {
				continue
			}
			sha := parts[0]
			ts, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				continue
			}
			subject := ""
			if len(parts) >= 3 {
				subject = parts[2]
			}

			currentEntry = &rawCommitEntry{
				commitSHA: sha,
				unixTime:  ts,
				subject:   subject,
			}
			continue
		}

		if strings.HasPrefix(line, ":") && currentEntry != nil {
			// Raw diff line: :<old-mode> <new-mode> <old-sha> <new-sha> <status>\t<path...>
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				currentEntry.oldBlob = fields[2]
				currentEntry.newBlob = fields[3]
				currentEntry.status = fields[4]

				// Path is separated by tabs after status
				tabParts := strings.Split(line, "\t")
				if len(tabParts) == 2 {
					currentEntry.newPath = tabParts[1]
				} else if len(tabParts) >= 3 {
					currentEntry.oldPath = tabParts[1]
					currentEntry.newPath = tabParts[2]
				}
			}
		}
	}

	if currentEntry != nil {
		entries = append(entries, *currentEntry)
	}

	return entries, nil
}

// batchFetchBlobs streams blob contents using git cat-file --batch and computes DocMetrics.
func batchFetchBlobs(ctx context.Context, repoDir string, blobSHAs []string) (map[string]DocMetrics, error) {
	results := make(map[string]DocMetrics, len(blobSHAs))
	if len(blobSHAs) == 0 {
		return results, nil
	}

	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	if repoDir != "" {
		cmd.Dir = repoDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cat-file: %w", err)
	}

	// Write blob SHAs in a goroutine
	go func() {
		defer stdin.Close()
		for _, sha := range blobSHAs {
			fmt.Fprintf(stdin, "%s\n", sha)
		}
	}()

	reader := bufio.NewReader(stdout)
	for range blobSHAs {
		header, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read header: %w", err)
		}

		header = strings.TrimSpace(header)
		parts := strings.Fields(header)
		if len(parts) >= 3 && parts[1] == "blob" {
			sha := parts[0]
			size, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse blob size: %w", err)
			}

			buf := make([]byte, size)
			if _, err := io.ReadFull(reader, buf); err != nil {
				return nil, fmt.Errorf("read blob content: %w", err)
			}

			// Read trailing newline
			if _, err := reader.ReadByte(); err != nil {
				return nil, fmt.Errorf("read trailing newline: %w", err)
			}

			results[sha] = ComputeDocMetrics(buf)
		} else if len(parts) >= 2 && parts[1] == "missing" {
			continue
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("wait cat-file: %w", err)
	}

	return results, nil
}

// SparklineGlyphs defines the 8-level Unicode lower block sparkline scale (U+2581 to U+2588).
var SparklineGlyphs = []rune("\u2581\u2582\u2583\u2584\u2585\u2586\u2587\u2588")

// RenderSparkline maps a slice of integer values to a Unicode sparkline.
func RenderSparkline(values []int, maxWidth int) string {
	if len(values) == 0 {
		return ""
	}

	if maxWidth > 0 && len(values) > maxWidth {
		values = values[len(values)-maxWidth:]
	}

	minVal := values[0]
	maxVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	numGlyphs := len(SparklineGlyphs)
	res := make([]rune, len(values))

	if maxVal == minVal {
		glyph := SparklineGlyphs[0]
		if maxVal > 0 {
			glyph = SparklineGlyphs[numGlyphs/2]
		}
		for i := range values {
			res[i] = glyph
		}
		return string(res)
	}

	span := float64(maxVal - minVal)
	for i, v := range values {
		idx := int((float64(v-minVal) / span) * float64(numGlyphs-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= numGlyphs {
			idx = numGlyphs - 1
		}
		res[i] = SparklineGlyphs[idx]
	}

	return string(res)
}

// RenderDocHistoryTable formats DocHistoryResult as a clean terminal table.
func RenderDocHistoryTable(res *DocHistoryResult) string {
	if res == nil || len(res.Snapshots) == 0 {
		return "No git history found for specified document.\n"
	}

	var b strings.Builder
	title := fmt.Sprintf("── Document Token Evolution: %s ──────────────────────────────", res.FilePath)
	b.WriteString(title + "\n")

	// Cache stats
	hitRate := 0.0
	totalAccesses := res.CacheHits + res.CacheMisses
	if totalAccesses > 0 {
		hitRate = float64(res.CacheHits) / float64(totalAccesses) * 100.0
	}
	b.WriteString(fmt.Sprintf("Commits: %d | Unique Blobs: %d | Cache Hits: %d (%.1f%%) | Latency: %v\n",
		res.TotalCommits, res.UniqueBlobs, res.CacheHits, hitRate, res.Duration.Round(time.Millisecond)))

	// Sparkline
	tokens := make([]int, len(res.Snapshots))
	for i, s := range res.Snapshots {
		tokens[i] = s.EstTokens
	}
	sparkline := RenderSparkline(tokens, 40)
	startTok := tokens[0]
	endTok := tokens[len(tokens)-1]
	diffTok := endTok - startTok
	pctChange := 0.0
	if startTok > 0 {
		pctChange = float64(diffTok) / float64(startTok) * 100.0
	}
	sign := "+"
	if diffTok < 0 {
		sign = ""
	}

	b.WriteString(fmt.Sprintf("Sparkline (Tokens): [%s]  (%s → %s tokens, %s%.1f%%)\n\n",
		sparkline, formatNumber(startTok), formatNumber(endTok), sign, pctChange))

	// Table Header
	b.WriteString(fmt.Sprintf("%-10s  %-8s  %-12s  %7s  %7s  %5s  %5s  %4s  %s\n",
		"DATE", "COMMIT", "DELTA (TOK)", "TOKENS", "BYTES", "LINES", "WORDS", "HEAD", "SUBJECT"))
	b.WriteString(strings.Repeat("─", 88) + "\n")

	prevTokens := 0
	for i, s := range res.Snapshots {
		dateStr := s.CommitTime.Format("2006-01-02")
		commitShort := s.CommitSHA
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}

		deltaStr := fmt.Sprintf("%+d", s.EstTokens)
		if i > 0 {
			diff := s.EstTokens - prevTokens
			deltaStr = fmt.Sprintf("%+d", diff)
		}

		subj := s.CommitSubject
		if len(subj) > 34 {
			subj = subj[:31] + "..."
		}

		b.WriteString(fmt.Sprintf("%-10s  %-8s  %11s   %7s  %7s  %5d  %5d  %4d  %s\n",
			dateStr,
			commitShort,
			deltaStr,
			formatNumber(s.EstTokens),
			formatNumber(int(s.RawBytes)),
			s.Lines,
			s.Words,
			s.Headings,
			subj,
		))
		prevTokens = s.EstTokens
	}

	return b.String()
}
