package assess

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileCategory classifies files into standard high-level roles.
type FileCategory string

const (
	CategoryCode   FileCategory = "Code"
	CategoryDocs   FileCategory = "Docs"
	CategoryTests  FileCategory = "Tests"
	CategoryConfig FileCategory = "Config"
	CategoryOther  FileCategory = "Other"
)

// LanguageInfo maps file extensions to language name and default category.
type LanguageInfo struct {
	Language string
	Category FileCategory
}

// FileMetrics holds line, token, and size statistics for a single file.
type FileMetrics struct {
	Path     string       `json:"path"`
	RelPath  string       `json:"rel_path"`
	Category FileCategory `json:"category"`
	Language string       `json:"language"`
	IsTest   bool         `json:"is_test"`
	IsDoc    bool         `json:"is_doc"`
	IsConfig bool         `json:"is_config"`
	Bytes    int64        `json:"bytes"`
	Lines    int          `json:"lines"`  // Non-blank lines of code/content
	Tokens   int          `json:"tokens"` // Estimated token count
	Warnings []string     `json:"warnings,omitempty"`
}

// CategorySummary aggregates metrics for a single category or language group.
type CategorySummary struct {
	Name       string       `json:"name"`
	Category   FileCategory `json:"category"`
	FilesCount int          `json:"files_count"`
	Lines      int          `json:"lines"`
	Tokens     int          `json:"tokens"`
	Bytes      int64        `json:"bytes"`
	Health     string       `json:"health,omitempty"`
}

// AssessmentReport is the top-level assessment data structure.
type AssessmentReport struct {
	Target          string            `json:"target"`
	IsFile          bool              `json:"is_file"`
	TotalFiles      int               `json:"total_files"`
	TotalLines      int               `json:"total_lines"`
	TotalTokens     int               `json:"total_tokens"`
	TotalBytes      int64             `json:"total_bytes"`
	Categories      []CategorySummary `json:"categories"`
	TopLanguages    []CategorySummary `json:"top_languages,omitempty"`
	Files           []FileMetrics     `json:"files,omitempty"`
	Feasibility     string            `json:"feasibility"`
	FeasibilityDesc string            `json:"feasibility_desc,omitempty"`
	Warnings        []string          `json:"warnings"`
	CodeToTestRatio float64           `json:"code_to_test_ratio,omitempty"`
	CodeToDocRatio  float64           `json:"code_to_doc_ratio,omitempty"`
	RAMP            *RAMPProfile      `json:"ramp,omitempty"`
}

// EstimateTokens calculates an approximate token count based on character length.
// Code and prose average ~3.5 to 4.0 characters per token across modern LLM tokenizers (Claude/GPT/Gemini).
func EstimateTokens(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	charCount := len(data)
	tokens := (charCount + 3) / 4 // Heuristic: ~3.75 chars per token
	if tokens < 1 && len(data) > 0 {
		return 1
	}
	return tokens
}

// CountLines returns the count of non-blank lines.
func CountLines(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	count := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 {
			count++
		}
	}
	return count
}

// InspectFile reads a file and computes its metrics and warnings.
func InspectFile(path, relPath string) (*FileMetrics, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, nil
	}

	// Ignore symlinks if broken or directory
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, nil
		}
		fi, err = os.Stat(target)
		if err != nil || fi.IsDir() {
			return nil, nil
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	if defaultIgnoredExts[ext] || fi.Size() > 10*1024*1024 {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if IsBinary(data) {
		return nil, nil
	}

	cat, lang, isTest, isDoc, isConfig := ClassifyFile(relPath)
	lines := CountLines(data)
	tokens := EstimateTokens(data)

	m := &FileMetrics{
		Path:     path,
		RelPath:  relPath,
		Category: cat,
		Language: lang,
		IsTest:   isTest,
		IsDoc:    isDoc,
		IsConfig: isConfig,
		Bytes:    fi.Size(),
		Lines:    lines,
		Tokens:   tokens,
	}

	// Check warnings per file
	// 1. Bash script warnings (>100-150 LOC)
	if lang == "Bash" && lines > 150 {
		m.Warnings = append(m.Warnings, "Bash script exceeds 150 LOC threshold (consider modularizing or Go rewrite per docs/Bash.md)")
	} else if lang == "Bash" && lines > 100 {
		m.Warnings = append(m.Warnings, "Bash script exceeds 100 LOC (approaching maintenance threshold)")
	}

	// 2. High LOC / TOC (>500 LOC or >3,500 tokens)
	if (cat == CategoryCode || cat == CategoryTests) && lines > 500 {
		m.Warnings = append(m.Warnings, "High LOC file (>500 LOC, consider splitting/modularizing)")
	}
	if tokens > 4000 {
		m.Warnings = append(m.Warnings, "High token weight (>4k tokens, agent context heavy)")
	}

	// 3. Special file warnings (e.g. bloated AGENTS.md > 250 LOC)
	base := filepath.Base(relPath)
	if (base == "AGENTS.md" || base == "CLAUDE.md") && lines > 250 {
		m.Warnings = append(m.Warnings, "Oversized agent convention doc (>250 LOC, prune redundant sections)")
	}

	return m, nil
}

// AssessPath assesses a single file or a directory tree.
func AssessPath(targetPath string) (*AssessmentReport, error) {
	fi, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		absTarget = targetPath
	}

	report := &AssessmentReport{
		Target: absTarget,
		IsFile: !fi.IsDir(),
	}

	if !fi.IsDir() {
		rel := filepath.Base(absTarget)
		m, err := InspectFile(absTarget, rel)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return report, nil
		}
		report.TotalFiles = 1
		report.TotalLines = m.Lines
		report.TotalTokens = m.Tokens
		report.TotalBytes = m.Bytes
		report.Files = []FileMetrics{*m}
		report.Warnings = append(report.Warnings, m.Warnings...)

		catSummary := CategorySummary{
			Name:       string(m.Category) + " (" + m.Language + ")",
			Category:   m.Category,
			FilesCount: 1,
			Lines:      m.Lines,
			Tokens:     m.Tokens,
			Bytes:      m.Bytes,
			Health:     "✓ Clean",
		}
		if len(m.Warnings) > 0 {
			catSummary.Health = "! Warning"
		}
		report.Categories = []CategorySummary{catSummary}
		report.Feasibility = "High"
		if len(report.Warnings) > 0 {
			report.Feasibility = "Moderate"
		}
		return report, nil
	}

	// Directory traversal
	var fileList []FileMetrics
	langMap := make(map[string]*CategorySummary)
	catMap := make(map[FileCategory]*CategorySummary)

	codeFilesPerDir := make(map[string]int)
	testFilesPerDir := make(map[string]int)

	err = filepath.WalkDir(absTarget, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if defaultIgnoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(absTarget, p)
		if err != nil {
			rel = p
		}

		m, err := InspectFile(p, rel)
		if err != nil || m == nil {
			return nil
		}

		fileList = append(fileList, *m)
		report.TotalFiles++
		report.TotalLines += m.Lines
		report.TotalTokens += m.Tokens
		report.TotalBytes += m.Bytes

		// Track code vs test directories
		dirOfFile := filepath.Dir(rel)
		if m.Category == CategoryCode {
			codeFilesPerDir[dirOfFile]++
		} else if m.Category == CategoryTests {
			testFilesPerDir[dirOfFile]++
		}

		// Language breakdown (for Code)
		groupKey := m.Language
		if m.Category == CategoryCode {
			groupKey = "Code (" + m.Language + ")"
		} else if m.Category == CategoryTests {
			groupKey = "Tests"
		} else if m.Category == CategoryDocs {
			groupKey = "Docs"
		} else if m.Category == CategoryConfig {
			groupKey = "Config"
		}

		if _, ok := langMap[groupKey]; !ok {
			langMap[groupKey] = &CategorySummary{
				Name:     groupKey,
				Category: m.Category,
			}
		}
		langMap[groupKey].FilesCount++
		langMap[groupKey].Lines += m.Lines
		langMap[groupKey].Tokens += m.Tokens
		langMap[groupKey].Bytes += m.Bytes

		if _, ok := catMap[m.Category]; !ok {
			catMap[m.Category] = &CategorySummary{
				Name:     string(m.Category),
				Category: m.Category,
			}
		}
		catMap[m.Category].FilesCount++
		catMap[m.Category].Lines += m.Lines
		catMap[m.Category].Tokens += m.Tokens
		catMap[m.Category].Bytes += m.Bytes

		for _, w := range m.Warnings {
			report.Warnings = append(report.Warnings, rel+": "+w)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	report.Files = fileList

	// Check directory-level missing test warnings
	for dir, codeCount := range codeFilesPerDir {
		if codeCount >= 3 && testFilesPerDir[dir] == 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Directory %s contains %d code files but no test files", dir, codeCount))
		}
	}

	// Calculate ratios
	var codeLines, testLines, docLines int
	if c, ok := catMap[CategoryCode]; ok {
		codeLines = c.Lines
	}
	if t, ok := catMap[CategoryTests]; ok {
		testLines = t.Lines
	}
	if d, ok := catMap[CategoryDocs]; ok {
		docLines = d.Lines
	}

	if codeLines > 0 {
		report.CodeToTestRatio = float64(testLines) / float64(codeLines)
		report.CodeToDocRatio = float64(docLines) / float64(codeLines)
	}

	// Convert langMap to sorted slice
	var categories []CategorySummary
	for _, cs := range langMap {
		// Calculate health
		switch cs.Category {
		case CategoryCode:
			if strings.Contains(cs.Name, "Bash") {
				if cs.FilesCount > 0 && cs.Lines/cs.FilesCount > 150 {
					cs.Health = "! High average LOC/file"
				} else {
					cs.Health = "✓ Clean (<150 LOC/file)"
				}
			} else {
				cs.Health = "✓ Healthy"
			}
		case CategoryTests:
			if report.CodeToTestRatio >= 0.3 {
				cs.Health = fmt.Sprintf("✓ Test ratio: %.0f%%", report.CodeToTestRatio*100)
			} else if report.CodeToTestRatio > 0 {
				cs.Health = fmt.Sprintf("~ Moderate test ratio (%.0f%%)", report.CodeToTestRatio*100)
			} else {
				cs.Health = "! No tests found"
			}
		case CategoryDocs:
			if docLines > 0 {
				cs.Health = "✓ Well-documented"
			} else {
				cs.Health = "! Sparse docs"
			}
		default:
			cs.Health = "✓ Clean"
		}
		categories = append(categories, *cs)
	}

	// Sort categories by lines descending
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Lines > categories[j].Lines
	})
	report.Categories = categories

	// Overall Feasibility evaluation
	if len(report.Warnings) == 0 && (report.CodeToTestRatio >= 0.2 || codeLines == 0) && report.TotalTokens < 200000 {
		report.Feasibility = "High"
		report.FeasibilityDesc = "modular, well-tested, within agent context budget"
	} else if len(report.Warnings) <= 3 && report.TotalTokens < 500000 {
		report.Feasibility = "Moderate"
		report.FeasibilityDesc = "maintainable with minor attention areas"
	} else {
		report.Feasibility = "Attention Needed"
		report.FeasibilityDesc = "elevated maintenance risks or token density"
	}

	report.RAMP, _ = AssessRAMP(absTarget)

	return report, nil
}
