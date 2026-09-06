# 254 — Splash: Brand Glyphs (Λ agy ✳ claude ֍ codex ● mic) with Green/Red Status Instead of Checkmarks

**Status**: In Progress — in progress — lean fresh-sprint
**Priority**: P2 (Medium)
**Severity**: Minor (UX polish / brand alignment)
**Category**: UX / Agentic Ergonomics
**Related**: [[252-splash-completed-source-badges-agy-claude-mic-under-fetch-status-log]] (shipped initial `✓ <source>` row),
[[169-splash-status-log-line-per-fetch-stage]],
`internal/usage/watch.go`, `internal/usage/watch_test.go`

---

## 1. Problem & Motivation

Ticket 252 added a completed probe badges row (`✓ agy  ✓ claude  ✓ mic`) below the rolling status log
line during `harnez usage --watch` startup splash.

While functional, the repetitive `✓` checkmark glyph adds unnecessary visual clutter. The user requested
replacing the uniform `✓` / `✗` prefix with dedicated, distinctive single-width brand glyphs where the
glyph itself is color-coded **green** on success and **red** on failure, with the source label in dim grey:
- **`agy`**: `Λ` (Antigravity arch / Gaussian curve, `U+039B`)
- **`claude`**: `✳` (Anthropic starburst / asterisk, `U+2733`)
- **`codex`**: `֍` (OpenAI / Codex rosette swirl, `U+058D`)
- **`gemini`**: `✦` (Google Gemini 4-point sparkle, `U+2726`)
- **`mic`**: `●` (Microphone recording dot, `U+25CF`)
- **`gpu`**: `⚙` (Compute gear, `U+2699`)
- **fallback**: `●`

## 2. Technical Design

### 2.1 Glyph Resolution Helper
In `internal/usage/watch.go`, introduce a helper function `splashSourceGlyph(source string) string`:
```go
func splashSourceGlyph(source string) string {
    switch source {
    case "agy":
        return "Λ"
    case "claude":
        return "✳"
    case "codex":
        return "֍"
    case "gemini":
        return "✦"
    case "mic":
        return "●"
    case "gpu", "load":
        return "⚙"
    default:
        return "●"
    }
}
```

### 2.2 Color-Coded Badge Formatting
Update `splashBadgesLine`:
- For each badge, format as: `ansiWrap(color, glyph) + " " + ansiWrap("dim-grey", source)`.
- If `badge.ok` is true, color is `"chart-green"`.
- If `badge.ok` is false, color is `"chart-warm"` (red).
- Badges are joined with `"  "` (two spaces).

Example Rendered Output:
```text
Λ agy   ✳ claude   ֍ codex   ● mic
```
(Where `Λ`, `✳`, `֍`, `●` are green/red and `agy`, `claude`, `codex`, `mic` are dim grey).

## 3. Scope of Implementation

1. **`internal/usage/watch.go`**:
   - Add `splashSourceGlyph` mapping.
   - Update `splashBadgesLine` to render colored brand glyphs + dim labels without `✓` / `✗`.
2. **`internal/usage/watch_test.go`**:
   - Update `TestSplashBadgesLine` and `TestBuildSplashFrameBadgesRow` for brand glyphs and green/red ANSI color assertions.
   - Verify that all visual length and centering assertions remain valid.

## 4. Acceptance Criteria

- [ ] Splash badges render with brand-specific glyphs (`Λ agy`, `✳ claude`, `֍ codex`, `● mic`).
- [ ] No `✓` or `✗` prefix is present; the glyph itself is styled with `chart-green` (success) or `chart-warm` (failure).
- [ ] Labels are rendered in `dim-grey`.
- [ ] All tests pass (`make check`) and binary installs cleanly (`make install`).

## 5. Verification

- `go test -v ./internal/usage/...`
- `make check`

