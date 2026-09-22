package assess

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TrackType represents one of the 5 functional tracks.
type TrackType string

const (
	TrackCode   TrackType = "Code"
	TrackTests  TrackType = "Tests"
	TrackDocs   TrackType = "Docs"
	TrackSkills TrackType = "Skills"
	TrackIssues TrackType = "Issues"
)

// AllTrackTypes lists the 5 functional tracks in standard display order.
var AllTrackTypes = []TrackType{
	TrackCode,
	TrackTests,
	TrackDocs,
	TrackSkills,
	TrackIssues,
}

// TrackMetrics holds aggregate volume and size for a specific track.
type TrackMetrics struct {
	Track         TrackType `json:"track"`
	Files         int       `json:"files"`
	Lines         int       `json:"lines"`
	Words         int       `json:"words"`
	Tokens        int       `json:"tokens"`
	Bytes         int64     `json:"bytes"`
	OpenTickets   int       `json:"open_tickets,omitempty"`
	ClosedTickets int       `json:"closed_tickets,omitempty"`
}

// TrackPoint represents metrics for a track at a specific commit snapshot.
type TrackPoint struct {
	CommitSHA  string    `json:"commit_sha"`
	CommitTime time.Time `json:"commit_time"`
	Lines      int       `json:"lines"`
	Tokens     int       `json:"tokens"`
	Files      int       `json:"files"`
	Bytes      int64     `json:"bytes"`
	Open       int       `json:"open,omitempty"`
	Closed     int       `json:"closed,omitempty"`
}

// TrackEvolution contains the time-series points, summary metrics, and sparkline for a single track.
type TrackEvolution struct {
	Track           TrackType    `json:"track"`
	CurrentFiles    int          `json:"current_files"`
	CurrentLines    int          `json:"current_lines"`
	CurrentTokens   int          `json:"current_tokens"`
	CurrentBytes    int64        `json:"current_bytes"`
	OpenTickets     int          `json:"open_tickets,omitempty"`
	ClosedTickets   int          `json:"closed_tickets,omitempty"`
	TotalAdded      int          `json:"total_added"`
	TotalRemoved    int          `json:"total_removed"`
	Sparkline       string       `json:"sparkline"`
	AddSparkline    string       `json:"add_sparkline,omitempty"`
	RemoveSparkline string       `json:"remove_sparkline,omitempty"`
	AddedPoints     []int        `json:"added_points,omitempty"`
	RemovedPoints   []int        `json:"removed_points,omitempty"`
	Points          []TrackPoint `json:"points"`
}

// MultiTrackHistoryResult is the complete multi-track repository evolution report.
type MultiTrackHistoryResult struct {
	RepoDir       string                        `json:"repo_dir"`
	RepoName      string                        `json:"repo_name"`
	TotalCommits  int                           `json:"total_commits"`
	SampleCommits int                           `json:"sample_commits"`
	DaysSpan      int                           `json:"days_span"`
	FirstCommit   time.Time                     `json:"first_commit"`
	LastCommit    time.Time                     `json:"last_commit"`
	TestCodeRatio float64                       `json:"test_code_ratio"`
	Tracks        map[TrackType]*TrackEvolution `json:"tracks"`
	Duration      time.Duration                 `json:"duration"`
}

// ClassifyTrack categorizes a git-relative file path into one of the 5 functional tracks.
// Returns (track, ok). If ok is false, the file does not belong to any tracked dimension (e.g. build artifacts, vendor, configs).
func ClassifyTrack(relPath string) (TrackType, bool) {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if clean == "." || clean == "" {
		return "", false
	}

	// Ignored directories / files
	parts := strings.Split(clean, "/")
	for _, p := range parts {
		if p == ".git" || p == "node_modules" || p == "vendor" || p == ".idea" || p == ".vscode" || p == "target" || p == "dist" || p == "build" || p == ".cache" {
			return "", false
		}
	}

	base := filepath.Base(clean)
	lowerBase := strings.ToLower(base)
	ext := strings.ToLower(filepath.Ext(base))

	// 5. Issues: ticket tracker markdown files (issues/*.md)
	if parts[0] == "issues" && strings.HasSuffix(lowerBase, ".md") && lowerBase != "readme.md" {
		return TrackIssues, true
	}

	// 4. Skills: agent skill declarations (skills/**/SKILL.md, docs/commands/*.md, commands/*.md, skills/**/*.md)
	if (parts[0] == "skills" && (lowerBase == "skill.md" || strings.HasSuffix(lowerBase, ".md"))) ||
		(parts[0] == "commands" && strings.HasSuffix(lowerBase, ".md")) ||
		(len(parts) >= 2 && parts[0] == "docs" && parts[1] == "commands" && strings.HasSuffix(lowerBase, ".md")) {
		return TrackSkills, true
	}

	// 2. Tests: test files
	if strings.HasSuffix(lowerBase, "_test.go") ||
		strings.HasSuffix(lowerBase, "_test.py") ||
		strings.HasPrefix(lowerBase, "test_") ||
		strings.HasSuffix(lowerBase, "_test.rs") ||
		strings.HasSuffix(lowerBase, ".test.js") ||
		strings.HasSuffix(lowerBase, ".test.ts") ||
		strings.HasSuffix(lowerBase, ".spec.js") ||
		strings.HasSuffix(lowerBase, ".spec.ts") ||
		parts[0] == "test" || parts[0] == "tests" ||
		strings.Contains(clean, "/test/") || strings.Contains(clean, "/tests/") ||
		strings.Contains(clean, "testdata/") {
		// Verify it's a code-like extension
		if isRecognizedCodeExt(ext) || isRecognizedCodeBase(base) {
			return TrackTests, true
		}
	}

	// 3. Docs: markdown / documentation excluding issues and skills
	if parts[0] == "docs" && (ext == ".md" || ext == ".txt" || ext == ".rst" || ext == ".adoc") {
		return TrackDocs, true
	}
	if lowerBase == "readme.md" || lowerBase == "claude.md" || lowerBase == "agents.md" || lowerBase == "agents.local.md" {
		return TrackDocs, true
	}
	if ext == ".md" || ext == ".markdown" {
		return TrackDocs, true
	}

	// 1. Code: source code files excluding tests
	if isRecognizedCodeExt(ext) || isRecognizedCodeBase(base) {
		return TrackCode, true
	}

	return "", false
}

func isRecognizedCodeExt(ext string) bool {
	switch ext {
	case ".go", ".py", ".rs", ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp",
		".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".sh", ".bash", ".zsh", ".zig", ".lua", ".rb", ".php",
		".java", ".kt", ".swift", ".scala", ".pl", ".pm", ".sql", ".css", ".scss", ".html":
		return true
	default:
		return false
	}
}

func isRecognizedCodeBase(base string) bool {
	switch base {
	case "Makefile", "makefile", "GNUmakefile":
		return true
	default:
		return false
	}
}

// TrackBlobInfo stores metadata and metrics for a specific blob SHA.
type TrackBlobInfo struct {
	Lines     int
	Tokens    int
	Bytes     int64
	Words     int
	IsOpen    bool
	IsClosed  bool
	HasStatus bool
}

// ExtractMultiTrackHistory walks git history and extracts multi-track evolution time series.
func ExtractMultiTrackHistory(repoDir string) (*MultiTrackHistoryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ExtractMultiTrackHistoryContext(ctx, repoDir)
}

type commitTrackDelta struct {
	sha     string
	time    time.Time
	subject string
	changes []rawTrackChange
}

type rawTrackChange struct {
	oldBlob string
	newBlob string
	status  string
	oldPath string
	newPath string
}

// ExtractMultiTrackHistoryContext walks git commit history with context support.
func ExtractMultiTrackHistoryContext(ctx context.Context, repoDir string) (*MultiTrackHistoryResult, error) {
	start := time.Now()

	// 1. Run git log --raw to extract full commit changes
	cmd := exec.CommandContext(ctx, "git", "log", "--raw", "--abbrev=40", "--format=commit %H %at %s")
	if repoDir != "" {
		cmd.Dir = repoDir
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	commits, allBlobs, err := parseMultiTrackLog(out)
	if err != nil {
		return nil, fmt.Errorf("parse git log: %w", err)
	}

	repoName := filepath.Base(repoDir)
	if repoName == "" || repoName == "." {
		if abs, err := filepath.Abs(repoDir); err == nil {
			repoName = filepath.Base(abs)
		}
	}

	if len(commits) == 0 {
		return &MultiTrackHistoryResult{
			RepoDir:  repoDir,
			RepoName: repoName,
			Tracks:   make(map[TrackType]*TrackEvolution),
			Duration: time.Since(start),
		}, nil
	}

	// 2. Batch fetch blob contents via git cat-file --batch
	blobInfoCache, err := batchFetchTrackBlobs(ctx, repoDir, allBlobs)
	if err != nil {
		return nil, fmt.Errorf("batch fetch blobs: %w", err)
	}

	// 3. Chronological commit replay (oldest to newest)
	// commits is newest to oldest
	totalCommits := len(commits)
	firstCommit := commits[totalCommits-1].time
	lastCommit := commits[0].time
	daysSpan := int(lastCommit.Sub(firstCommit).Hours()/24) + 1

	// Active file states per track: track -> path -> blobSHA
	activeFiles := make(map[TrackType]map[string]string)
	for _, t := range AllTrackTypes {
		activeFiles[t] = make(map[string]string)
	}

	// Determine sampling points across history (aim for up to ~30-50 sample points for sparklines)
	sampleStride := 1
	if totalCommits > 40 {
		sampleStride = totalCommits / 30
		if sampleStride < 1 {
			sampleStride = 1
		}
	}

	pointsByTrack := make(map[TrackType][]TrackPoint)
	for _, t := range AllTrackTypes {
		pointsByTrack[t] = make([]TrackPoint, 0, 40)
	}

	recordSample := func(c commitTrackDelta) {
		for _, track := range AllTrackTypes {
			var totLines, totTokens, totFiles, openCount, closedCount int
			var totBytes int64

			for path, blob := range activeFiles[track] {
				info, ok := blobInfoCache[blob]
				if !ok {
					continue
				}
				totFiles++
				totLines += info.Lines
				totTokens += info.Tokens
				totBytes += info.Bytes

				if track == TrackIssues {
					if info.HasStatus {
						if info.IsOpen {
							openCount++
						} else if info.IsClosed {
							closedCount++
						}
					} else {
						// Default heuristics: check path or assume open
						if strings.Contains(path, "closed") {
							closedCount++
						} else {
							openCount++
						}
					}
				}
			}

			pt := TrackPoint{
				CommitSHA:  c.sha,
				CommitTime: c.time,
				Lines:      totLines,
				Tokens:     totTokens,
				Files:      totFiles,
				Bytes:      totBytes,
				Open:       openCount,
				Closed:     closedCount,
			}
			pointsByTrack[track] = append(pointsByTrack[track], pt)
		}
	}

	// Replay from commit (N-1) down to 0
	for i := totalCommits - 1; i >= 0; i-- {
		c := commits[i]

		for _, ch := range c.changes {
			// Check old path for deletion/rename
			if ch.oldPath != "" {
				if oldTrack, ok := ClassifyTrack(ch.oldPath); ok {
					delete(activeFiles[oldTrack], ch.oldPath)
				}
			}

			// Check new path for add/modify/rename target
			if ch.newPath != "" && ch.newBlob != "" && ch.newBlob != "0000000000000000000000000000000000000000" {
				if newTrack, ok := ClassifyTrack(ch.newPath); ok {
					activeFiles[newTrack][ch.newPath] = ch.newBlob
				}
			}
		}

		// Sample if stride matched or last commit
		commitIdxFromStart := (totalCommits - 1) - i
		if commitIdxFromStart%sampleStride == 0 || i == 0 {
			recordSample(c)
		}
	}

	// Build TrackEvolution results
	tracks := make(map[TrackType]*TrackEvolution)
	for _, track := range AllTrackTypes {
		pts := pointsByTrack[track]
		var curFiles, curLines, curTokens, curOpen, curClosed int
		var curBytes int64

		if len(pts) > 0 {
			latest := pts[len(pts)-1]
			curFiles = latest.Files
			curLines = latest.Lines
			curTokens = latest.Tokens
			curBytes = latest.Bytes
			curOpen = latest.Open
			curClosed = latest.Closed
		}

		// Primary metric values per track
		rawValues := make([]int, len(pts))
		for j, p := range pts {
			switch track {
			case TrackCode, TrackTests:
				rawValues[j] = p.Lines
			case TrackDocs:
				rawValues[j] = p.Tokens
			case TrackSkills, TrackIssues:
				rawValues[j] = p.Files
			default:
				rawValues[j] = p.Lines
			}
		}

		addedPoints := make([]int, len(pts))
		removedPoints := make([]int, len(pts))
		if len(pts) > 0 {
			addedPoints[0] = rawValues[0]
			removedPoints[0] = 0
			for j := 0; j < len(pts)-1; j++ {
				delta := rawValues[j+1] - rawValues[j]
				if delta > 0 {
					addedPoints[j+1] = addedPoints[j] + delta
					removedPoints[j+1] = removedPoints[j]
				} else if delta < 0 {
					addedPoints[j+1] = addedPoints[j]
					removedPoints[j+1] = removedPoints[j] + (-delta)
				} else {
					addedPoints[j+1] = addedPoints[j]
					removedPoints[j+1] = removedPoints[j]
				}
			}
		}

		var totAdded, totRemoved int
		if len(pts) > 0 {
			totAdded = addedPoints[len(pts)-1]
			totRemoved = removedPoints[len(pts)-1]
		}

		sparkValues := make([]float64, len(pts))
		addFloats := make([]float64, len(pts))
		removeFloats := make([]float64, len(pts))
		for j := range pts {
			sparkValues[j] = float64(rawValues[j])
			addFloats[j] = float64(addedPoints[j])
			removeFloats[j] = float64(removedPoints[j])
		}

		spark := RenderBrailleSparkline(sparkValues, BrailleOptions{Width: 10, Color: false})
		addSpark := RenderBrailleSparkline(addFloats, BrailleOptions{Width: 10, Color: false})
		removeSpark := RenderBrailleSparkline(removeFloats, BrailleOptions{Width: 10, Color: false, InvertColor: true})

		tracks[track] = &TrackEvolution{
			Track:           track,
			CurrentFiles:    curFiles,
			CurrentLines:    curLines,
			CurrentTokens:   curTokens,
			CurrentBytes:    curBytes,
			OpenTickets:     curOpen,
			ClosedTickets:   curClosed,
			TotalAdded:      totAdded,
			TotalRemoved:    totRemoved,
			Sparkline:       spark,
			AddSparkline:    addSpark,
			RemoveSparkline: removeSpark,
			AddedPoints:     addedPoints,
			RemovedPoints:   removedPoints,
			Points:          pts,
		}
	}

	testCodeRatio := 0.0
	if codeTrack, ok := tracks[TrackCode]; ok && codeTrack.CurrentLines > 0 {
		if testTrack, ok := tracks[TrackTests]; ok {
			testCodeRatio = float64(testTrack.CurrentLines) / float64(codeTrack.CurrentLines)
		}
	}

	return &MultiTrackHistoryResult{
		RepoDir:       repoDir,
		RepoName:      repoName,
		TotalCommits:  totalCommits,
		SampleCommits: len(pointsByTrack[TrackCode]),
		DaysSpan:      daysSpan,
		FirstCommit:   firstCommit,
		LastCommit:    lastCommit,
		TestCodeRatio: testCodeRatio,
		Tracks:        tracks,
		Duration:      time.Since(start),
	}, nil
}

func parseMultiTrackLog(output []byte) ([]commitTrackDelta, []string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var commits []commitTrackDelta
	var current *commitTrackDelta
	blobSet := make(map[string]bool)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "commit ") {
			if current != nil {
				commits = append(commits, *current)
				current = nil
			}

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

			current = &commitTrackDelta{
				sha:     sha,
				time:    time.Unix(ts, 0).UTC(),
				subject: subject,
			}
			continue
		}

		if strings.HasPrefix(line, ":") && current != nil {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				oldBlob := fields[2]
				newBlob := fields[3]
				status := fields[4]

				tabParts := strings.Split(line, "\t")
				var oldPath, newPath string
				if len(tabParts) == 2 {
					if strings.HasPrefix(status, "D") {
						oldPath = tabParts[1]
					} else if strings.HasPrefix(status, "A") {
						newPath = tabParts[1]
					} else {
						oldPath = tabParts[1]
						newPath = tabParts[1]
					}
				} else if len(tabParts) >= 3 {
					oldPath = tabParts[1]
					newPath = tabParts[2]
				}

				if newBlob != "" && newBlob != "0000000000000000000000000000000000000000" {
					blobSet[newBlob] = true
				}

				current.changes = append(current.changes, rawTrackChange{
					oldBlob: oldBlob,
					newBlob: newBlob,
					status:  status,
					oldPath: oldPath,
					newPath: newPath,
				})
			}
		}
	}

	if current != nil {
		commits = append(commits, *current)
	}

	var blobs []string
	for b := range blobSet {
		blobs = append(blobs, b)
	}
	sort.Strings(blobs)

	return commits, blobs, nil
}

func batchFetchTrackBlobs(ctx context.Context, repoDir string, blobSHAs []string) (map[string]TrackBlobInfo, error) {
	results := make(map[string]TrackBlobInfo, len(blobSHAs))
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
			break
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
			if _, err := ioReadFull(reader, buf); err != nil {
				return nil, fmt.Errorf("read blob content: %w", err)
			}

			if _, err := reader.ReadByte(); err != nil {
				return nil, fmt.Errorf("read trailing newline: %w", err)
			}

			// Parse content
			lines := CountLines(buf)
			tokens := EstimateTokens(buf)
			words := CountWords(buf)
			isOpen, isClosed, hasStatus := parseTicketStatus(buf)

			results[sha] = TrackBlobInfo{
				Lines:     lines,
				Tokens:    tokens,
				Bytes:     size,
				Words:     words,
				IsOpen:    isOpen,
				IsClosed:  isClosed,
				HasStatus: hasStatus,
			}
		} else if len(parts) >= 2 && parts[1] == "missing" {
			continue
		}
	}

	_ = cmd.Wait()
	return results, nil
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		nn, err := r.Read(buf[n:])
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func parseTicketStatus(buf []byte) (isOpen bool, isClosed bool, hasStatus bool) {
	// Quick check in first 1500 bytes for "**Status**:" or "Status:"
	limit := len(buf)
	if limit > 1500 {
		limit = 1500
	}
	head := string(buf[:limit])
	lines := strings.Split(head, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "**Status**:") || strings.HasPrefix(trimmed, "Status:") {
			hasStatus = true
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "closed") || strings.Contains(lower, "done") || strings.Contains(lower, "resolved") || strings.Contains(lower, "wontfix") || strings.Contains(lower, "superseded") {
				isClosed = true
			} else {
				isOpen = true
			}
			return
		}
	}
	return false, false, false
}

// RenderTracksOptions configures multi-track rendering functions.
type RenderTracksOptions struct {
	Color bool
}

// RenderMultiTrackCardOptions configures RenderMultiTrackCard.
type RenderMultiTrackCardOptions struct {
	Color bool
	Diff  bool
}

// RenderMultiTrackCard formats the MultiTrackHistoryResult into a clean terminal card.
func RenderMultiTrackCard(res *MultiTrackHistoryResult, opts ...RenderMultiTrackCardOptions) string {
	if res == nil {
		return ""
	}

	useColor := false
	if len(opts) > 0 {
		useColor = opts[0].Color
		if opts[0].Diff {
			return RenderMultiTrackHistoryTableWithDiffs(res, RenderTracksOptions{Color: useColor})
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repo Evolution (%s · %d commits · %dd)\n", res.RepoName, res.TotalCommits, res.DaysSpan))

	for _, track := range AllTrackTypes {
		te, ok := res.Tracks[track]
		if !ok || te == nil {
			continue
		}

		spark := te.Sparkline
		if useColor && len(te.Points) > 0 {
			sparkValues := make([]float64, len(te.Points))
			for j, p := range te.Points {
				if track == TrackIssues {
					sparkValues[j] = float64(p.Files)
				} else if track == TrackCode || track == TrackTests {
					sparkValues[j] = float64(p.Lines)
				} else {
					sparkValues[j] = float64(p.Tokens)
				}
			}
			spark = RenderBrailleSparkline(sparkValues, BrailleOptions{Width: 10, Color: true})
		}
		if spark == "" {
			spark = strings.Repeat(" ", 10)
		}

		switch track {
		case TrackCode:
			locStr := formatK(te.CurrentLines)
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("%-8s [%s] %s LOC (%s tokens)\n", "Code:", spark, locStr, tokStr))
		case TrackTests:
			locStr := formatK(te.CurrentLines)
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("%-8s [%s] %s LOC (%s tokens) · %.2f test/code ratio\n", "Tests:", spark, locStr, tokStr, res.TestCodeRatio))
		case TrackDocs:
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("%-8s [%s] %s tokens (%d files)\n", "Docs:", spark, tokStr, te.CurrentFiles))
		case TrackSkills:
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("%-8s [%s] %d skills (%s tokens)\n", "Skills:", spark, te.CurrentFiles, tokStr))
		case TrackIssues:
			totalTickets := te.CurrentFiles
			sb.WriteString(fmt.Sprintf("%-8s [%s] %d tickets (%d open · %d closed)\n", "Issues:", spark, totalTickets, te.OpenTickets, te.ClosedTickets))
		}
	}

	return sb.String()
}

func formatK(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	val := float64(n) / 1000.0
	if val >= 100 {
		return fmt.Sprintf("%.0fk", val)
	}
	return fmt.Sprintf("%.1fk", val)
}

// RenderMultiTrackHistoryTableWithDiffs formats the MultiTrackHistoryResult into an expanded additions/removals breakdown table.
func RenderMultiTrackHistoryTableWithDiffs(result *MultiTrackHistoryResult, opts RenderTracksOptions) string {
	if result == nil {
		return ""
	}

	useColor := opts.Color
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repo Evolution Diffs (%s · %d commits · %dd)\n", result.RepoName, result.TotalCommits, result.DaysSpan))

	plusTag := "[+]"
	minusTag := "[-]"
	eqTag := "[=]"
	if useColor {
		plusTag = "\x1b[32m[+]\x1b[0m"
		minusTag = "\x1b[31m[-]\x1b[0m"
	}

	for _, track := range AllTrackTypes {
		te, ok := result.Tracks[track]
		if !ok || te == nil {
			continue
		}

		addSpark := te.AddSparkline
		removeSpark := te.RemoveSparkline
		netSpark := te.Sparkline

		if useColor && len(te.Points) > 0 {
			rawValues := make([]int, len(te.Points))
			for j, p := range te.Points {
				switch track {
				case TrackCode, TrackTests:
					rawValues[j] = p.Lines
				case TrackDocs:
					rawValues[j] = p.Tokens
				case TrackSkills, TrackIssues:
					rawValues[j] = p.Files
				default:
					rawValues[j] = p.Lines
				}
			}

			addFloats := make([]float64, len(te.AddedPoints))
			for j, v := range te.AddedPoints {
				addFloats[j] = float64(v)
			}
			removeFloats := make([]float64, len(te.RemovedPoints))
			for j, v := range te.RemovedPoints {
				removeFloats[j] = float64(v)
			}
			sparkValues := make([]float64, len(rawValues))
			for j, v := range rawValues {
				sparkValues[j] = float64(v)
			}

			addSpark = RenderBrailleSparkline(addFloats, BrailleOptions{Width: 10, Color: true})
			removeSpark = RenderBrailleSparkline(removeFloats, BrailleOptions{Width: 10, Color: true, InvertColor: true})
			netSpark = RenderBrailleSparkline(sparkValues, BrailleOptions{Width: 10, Color: true})
		}

		if addSpark == "" {
			addSpark = strings.Repeat(" ", 10)
		}
		if removeSpark == "" {
			removeSpark = strings.Repeat(" ", 10)
		}
		if netSpark == "" {
			netSpark = strings.Repeat(" ", 10)
		}

		var netVal int
		var unit string
		switch track {
		case TrackCode, TrackTests:
			netVal = te.CurrentLines
			unit = "LOC"
		case TrackDocs:
			netVal = te.CurrentTokens
			unit = "tokens"
		case TrackSkills:
			netVal = te.CurrentFiles
			unit = "skills"
		case TrackIssues:
			netVal = te.CurrentFiles
			unit = "tickets"
		}

		sAdd := "+" + formatK(te.TotalAdded)
		sRms := "0"
		if te.TotalRemoved > 0 {
			sRms = "-" + formatK(te.TotalRemoved)
		}
		sNet := formatK(netVal)

		w := len(sAdd)
		if len(sRms) > w {
			w = len(sRms)
		}
		if len(sNet) > w {
			w = len(sNet)
		}

		fmtAdd := fmt.Sprintf("%*s", w, sAdd)
		fmtRms := fmt.Sprintf("%*s", w, sRms)
		fmtNet := fmt.Sprintf("%*s", w, sNet)

		if useColor {
			fmtAdd = "\x1b[32m" + fmtAdd + "\x1b[0m"
			fmtRms = "\x1b[31m" + fmtRms + "\x1b[0m"
		}

		sb.WriteString(fmt.Sprintf("%s:\n", string(track)))
		sb.WriteString(fmt.Sprintf("  %s Adds: [%s] %s %s\n", plusTag, addSpark, fmtAdd, unit))
		sb.WriteString(fmt.Sprintf("  %s Rms:  [%s] %s %s\n", minusTag, removeSpark, fmtRms, unit))

		switch track {
		case TrackCode:
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("  %s Net:  [%s] %s %s (%s tokens)\n", eqTag, netSpark, fmtNet, unit, tokStr))
		case TrackTests:
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("  %s Net:  [%s] %s %s (%s tokens) · %.2f test/code ratio\n", eqTag, netSpark, fmtNet, unit, tokStr, result.TestCodeRatio))
		case TrackDocs:
			sb.WriteString(fmt.Sprintf("  %s Net:  [%s] %s %s (%d files)\n", eqTag, netSpark, fmtNet, unit, te.CurrentFiles))
		case TrackSkills:
			tokStr := formatK(te.CurrentTokens)
			sb.WriteString(fmt.Sprintf("  %s Net:  [%s] %s %s (%s tokens)\n", eqTag, netSpark, fmtNet, unit, tokStr))
		case TrackIssues:
			sb.WriteString(fmt.Sprintf("  %s Net:  [%s] %s %s (%d open · %d closed)\n", eqTag, netSpark, fmtNet, unit, te.OpenTickets, te.ClosedTickets))
		}
	}

	return sb.String()
}
