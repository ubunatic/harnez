package assess

import (
	"bytes"
	"path/filepath"
	"strings"
)

// Ignored directories during walk
var defaultIgnoredDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	".idea":        true,
	".vscode":      true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
}

// Ignored file extensions / artifacts
var defaultIgnoredExts = map[string]bool{
	".exe":   true,
	".bin":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".o":     true,
	".a":     true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".ico":   true,
	".webp":  true,
	".pdf":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".bz2":   true,
	".xz":    true,
	".7z":    true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".pyc":   true,
	".pyo":   true,
	".class": true,
}

var knownExts = map[string]LanguageInfo{
	// Go
	".go": {Language: "Go", Category: CategoryCode},
	// Bash / Shell
	".sh":   {Language: "Bash", Category: CategoryCode},
	".bash": {Language: "Bash", Category: CategoryCode},
	".zsh":  {Language: "Bash", Category: CategoryCode},
	// Python
	".py": {Language: "Python", Category: CategoryCode},
	// Rust
	".rs": {Language: "Rust", Category: CategoryCode},
	// C / C++
	".c":   {Language: "C", Category: CategoryCode},
	".h":   {Language: "C", Category: CategoryCode},
	".cpp": {Language: "C++", Category: CategoryCode},
	".cc":  {Language: "C++", Category: CategoryCode},
	".cxx": {Language: "C++", Category: CategoryCode},
	".hpp": {Language: "C++", Category: CategoryCode},
	// Zig
	".zig": {Language: "Zig", Category: CategoryCode},
	// JavaScript / TypeScript
	".js":  {Language: "JavaScript", Category: CategoryCode},
	".mjs": {Language: "JavaScript", Category: CategoryCode},
	".cjs": {Language: "JavaScript", Category: CategoryCode},
	".jsx": {Language: "JavaScript", Category: CategoryCode},
	".ts":  {Language: "TypeScript", Category: CategoryCode},
	".tsx": {Language: "TypeScript", Category: CategoryCode},
	// Web
	".html": {Language: "HTML", Category: CategoryCode},
	".htm":  {Language: "HTML", Category: CategoryCode},
	".css":  {Language: "CSS", Category: CategoryCode},
	".scss": {Language: "SCSS", Category: CategoryCode},
	".sass": {Language: "Sass", Category: CategoryCode},
	".less": {Language: "Less", Category: CategoryCode},
	// Ruby, PHP, Java, Kotlin, Swift
	".rb":    {Language: "Ruby", Category: CategoryCode},
	".php":   {Language: "PHP", Category: CategoryCode},
	".java":  {Language: "Java", Category: CategoryCode},
	".kt":    {Language: "Kotlin", Category: CategoryCode},
	".swift": {Language: "Swift", Category: CategoryCode},
	// Docs
	".md":       {Language: "Markdown", Category: CategoryDocs},
	".markdown": {Language: "Markdown", Category: CategoryDocs},
	".rst":      {Language: "reStructuredText", Category: CategoryDocs},
	".txt":      {Language: "Text", Category: CategoryDocs},
	".adoc":     {Language: "AsciiDoc", Category: CategoryDocs},
	// Configs
	".yaml":         {Language: "YAML", Category: CategoryConfig},
	".yml":          {Language: "YAML", Category: CategoryConfig},
	".json":         {Language: "JSON", Category: CategoryConfig},
	".jsonc":        {Language: "JSONC", Category: CategoryConfig},
	".toml":         {Language: "TOML", Category: CategoryConfig},
	".ini":          {Language: "INI", Category: CategoryConfig},
	".env":          {Language: "Config", Category: CategoryConfig},
	".editorconfig": {Language: "Config", Category: CategoryConfig},
}

// Special exact file name classifications
var specialFiles = map[string]LanguageInfo{
	"Makefile":      {Language: "Make", Category: CategoryCode},
	"makefile":      {Language: "Make", Category: CategoryCode},
	"GNUmakefile":   {Language: "Make", Category: CategoryCode},
	"Dockerfile":    {Language: "Docker", Category: CategoryConfig},
	"Containerfile": {Language: "Docker", Category: CategoryConfig},
	"AGENTS.md":     {Language: "Markdown", Category: CategoryDocs},
	"CLAUDE.md":     {Language: "Markdown", Category: CategoryDocs},
	"LICENSE":       {Language: "Text", Category: CategoryDocs},
	"README":        {Language: "Text", Category: CategoryDocs},
}

// IsBinary detects if the given byte slice represents binary content.
func IsBinary(data []byte) bool {
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	if limit == 0 {
		return false
	}
	// Check for null bytes
	if bytes.IndexByte(data[:limit], 0) != -1 {
		return true
	}
	// Check ratio of control characters
	controlChars := 0
	for _, b := range data[:limit] {
		if b < 7 || (b > 13 && b < 27) || (b > 27 && b < 32) {
			controlChars++
		}
	}
	return float64(controlChars)/float64(limit) > 0.30
}

// ClassifyFile determines the category, language, and sub-attributes of a file.
func ClassifyFile(relPath string) (category FileCategory, language string, isTest, isDoc, isConfig bool) {
	base := filepath.Base(relPath)
	lowerBase := strings.ToLower(base)
	ext := strings.ToLower(filepath.Ext(base))

	// Check special files
	if info, ok := specialFiles[base]; ok {
		category = info.Category
		language = info.Language
		if category == CategoryDocs {
			isDoc = true
		} else if category == CategoryConfig {
			isConfig = true
		}
		return
	}

	// Known extension
	if info, ok := knownExts[ext]; ok {
		category = info.Category
		language = info.Language
	} else if strings.HasPrefix(base, ".") && (strings.HasSuffix(base, "rc") || strings.HasSuffix(base, "config")) {
		category = CategoryConfig
		language = "Config"
	} else {
		category = CategoryOther
		language = "Other"
	}

	// Test detection
	if strings.HasSuffix(lowerBase, "_test.go") ||
		strings.HasSuffix(lowerBase, "_test.py") ||
		strings.HasPrefix(lowerBase, "test_") ||
		strings.HasSuffix(lowerBase, ".test.js") ||
		strings.HasSuffix(lowerBase, ".test.ts") ||
		strings.HasSuffix(lowerBase, ".spec.js") ||
		strings.HasSuffix(lowerBase, ".spec.ts") ||
		strings.Contains(relPath, "/test/") ||
		strings.Contains(relPath, "/tests/") ||
		strings.Contains(relPath, "testdata/") {
		if category == CategoryCode {
			isTest = true
			category = CategoryTests
		}
	}

	// Docs detection (e.g. inside docs/ directory)
	if strings.HasPrefix(relPath, "docs/") || strings.Contains(relPath, "/docs/") || category == CategoryDocs {
		isDoc = true
		category = CategoryDocs
	}

	if category == CategoryConfig {
		isConfig = true
	}

	return
}
