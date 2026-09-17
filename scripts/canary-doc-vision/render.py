#!/usr/bin/env python3
"""
render.py - Markdown to Document Screenshot Renderer & Token Efficiency Benchmark

Converts markdown documentation into styled HTML and renders high-resolution
PNG / JPG screenshots using headless Chromium, then calculates token efficiency
metrics across multimodal LLM harnesses (Claude, OpenAI/Codex, Gemini).
"""

import argparse
import html
import json
import math
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path


def parse_markdown_to_html(md_text: str) -> str:
    """Lightweight pure-Python markdown parser for technical docs."""
    lines = md_text.splitlines()
    html_out = []
    in_code_block = False
    code_lang = ""
    code_lines = []
    in_list = False
    list_type = "ul"
    in_table = False
    table_headers = []

    def flush_list():
        nonlocal in_list, list_type
        if in_list:
            html_out.append(f"</{list_type}>")
            in_list = False

    def flush_table():
        nonlocal in_table, table_headers
        if in_table:
            html_out.append("</tbody></table></div>")
            in_table = False
            table_headers = []

    def format_inline(text: str) -> str:
        # Escape HTML entities first (preserve existing backticks)
        text = html.escape(text)
        # Inline code: `code`
        text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
        # Bold: **bold** or __bold__
        text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
        text = re.sub(r"__([^_]+)__", r"<strong>\1</strong>", text)
        # Italic: *italic* or _italic_
        text = re.sub(r"\*([^*]+)\*", r"<em>\1</em>", text)
        text = re.sub(r"(?<!\w)_([^_]+)_(?!\w)", r"<em>\1</em>", text)
        # Markdown links: [text](url)
        text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<span class="link">\1</span>', text)
        return text

    for line in lines:
        stripped = line.strip()

        # Fenced code block handling
        if stripped.startswith("```"):
            if in_code_block:
                flush_list()
                code_content = "\n".join(code_lines)
                escaped = html.escape(code_content)
                lang_attr = f' class="language-{code_lang}"' if code_lang else ""
                html_out.append(f'<pre><code{lang_attr}>{escaped}</code></pre>')
                in_code_block = False
                code_lines = []
                code_lang = ""
            else:
                flush_list()
                flush_table()
                in_code_block = True
                code_lang = stripped[3:].strip()
                code_lines = []
            continue

        if in_code_block:
            code_lines.append(line)
            continue

        # Blank line
        if not stripped:
            flush_list()
            flush_table()
            continue

        # GitHub Alert / Callout block: > [!NOTE] or > [!WARNING]
        alert_match = re.match(r"^>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*)$", stripped, re.IGNORECASE)
        if alert_match:
            flush_list()
            flush_table()
            kind = alert_match.group(1).upper()
            rest = alert_match.group(2)
            html_out.append(f'<div class="callout callout-{kind.lower()}">')
            html_out.append(f'<div class="callout-title">{kind}</div>')
            if rest:
                html_out.append(f'<p>{format_inline(rest)}</p>')
            html_out.append('</div>')
            continue

        # Standard Blockquote
        if stripped.startswith(">"):
            flush_list()
            flush_table()
            quote_text = stripped[1:].strip()
            html_out.append(f'<blockquote>{format_inline(quote_text)}</blockquote>')
            continue

        # Horizontal Rule
        if re.match(r"^(-{3,}|\*{3,}|_{3,})$", stripped):
            flush_list()
            flush_table()
            html_out.append("<hr/>")
            continue

        # Headings
        heading_match = re.match(r"^(#{1,6})\s+(.*)$", stripped)
        if heading_match:
            flush_list()
            flush_table()
            level = len(heading_match.group(1))
            heading_text = format_inline(heading_match.group(2))
            html_out.append(f"<h{level}>{heading_text}</h{level}>")
            continue

        # Tables: | col1 | col2 | ...
        if stripped.startswith("|") and stripped.endswith("|"):
            flush_list()
            parts = [p.strip() for p in stripped[1:-1].split("|")]
            # Check if this is separator line: |---|---|
            if all(re.match(r"^:?-+:?$", p) for p in parts):
                continue  # Header separator
            if not in_table:
                in_table = True
                table_headers = parts
                html_out.append('<div class="table-wrapper"><table><thead><tr>')
                for h in table_headers:
                    html_out.append(f"<th>{format_inline(h)}</th>")
                html_out.append("</tr></thead><tbody>")
            else:
                html_out.append("<tr>")
                for cell in parts:
                    html_out.append(f"<td>{format_inline(cell)}</td>")
                html_out.append("</tr>")
            continue
        else:
            flush_table()

        # Unordered List Items: - item or * item
        list_match = re.match(r"^(\s*)[-*+]\s+(.*)$", line)
        if list_match:
            indent = len(list_match.group(1))
            item_text = format_inline(list_match.group(2))
            if not in_list or list_type != "ul":
                flush_list()
                in_list = True
                list_type = "ul"
                html_out.append("<ul>")
            html_out.append(f"<li>{item_text}</li>")
            continue

        # Ordered List Items: 1. item
        num_list_match = re.match(r"^(\s*)\d+\.\s+(.*)$", line)
        if num_list_match:
            item_text = format_inline(num_list_match.group(2))
            if not in_list or list_type != "ol":
                flush_list()
                in_list = True
                list_type = "ol"
                html_out.append("<ol>")
            html_out.append(f"<li>{item_text}</li>")
            continue

        # Paragraph
        flush_list()
        flush_table()
        html_out.append(f"<p>{format_inline(stripped)}</p>")

    if in_code_block:
        code_content = "\n".join(code_lines)
        escaped = html.escape(code_content)
        html_out.append(f'<pre><code>{escaped}</code></pre>')

    flush_list()
    flush_table()
    return "\n".join(html_out)


LAYOUT_STYLES = {
    "1col": {
        "name": "Standard 1-Column",
        "description": "Standard readable layout, 1200px width, 14px font, light/dark neutral theme",
        "width": 1200,
        "css": """
:root {
  --bg: #0d1117;
  --fg: #c9d1d9;
  --heading: #58a6ff;
  --border: #30363d;
  --code-bg: #161b22;
  --code-fg: #f0883e;
  --callout-bg: #1f242c;
  --link: #79c0ff;
  --font-base: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  --font-mono: "JetBrains Mono", "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}
body {
  margin: 0;
  padding: 32px 48px;
  background-color: var(--bg);
  color: var(--fg);
  font-family: var(--font-base);
  font-size: 14px;
  line-height: 1.6;
}
.container {
  max-width: 1104px;
  margin: 0 auto;
}
h1 { font-size: 26px; color: var(--heading); border-bottom: 2px solid var(--border); padding-bottom: 8px; margin-top: 0; }
h2 { font-size: 20px; color: var(--heading); border-bottom: 1px solid var(--border); padding-bottom: 6px; margin-top: 24px; }
h3 { font-size: 16px; color: #79c0ff; margin-top: 18px; }
h4, h5, h6 { font-size: 14px; color: #a5d6ff; }
p, li { margin: 6px 0; }
ul, ol { padding-left: 24px; }
code {
  font-family: var(--font-mono);
  font-size: 12.5px;
  background: var(--code-bg);
  color: var(--code-fg);
  padding: 2px 5px;
  border-radius: 4px;
  border: 1px solid var(--border);
}
pre {
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px 16px;
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.45;
  margin: 12px 0;
}
pre code { background: none; border: none; padding: 0; color: #e6edf3; }
blockquote {
  border-left: 4px solid var(--border);
  margin: 12px 0;
  padding: 4px 16px;
  color: #8b949e;
}
hr { border: 0; border-top: 1px solid var(--border); margin: 24px 0; }
.callout {
  border-left: 4px solid #58a6ff;
  background: var(--callout-bg);
  padding: 10px 14px;
  margin: 12px 0;
  border-radius: 4px;
}
.callout-warning, .callout-caution { border-left-color: #d29922; background: #272115; }
.callout-important { border-left-color: #bc8cff; background: #261f30; }
.callout-title { font-weight: bold; font-size: 12px; text-transform: uppercase; color: #58a6ff; margin-bottom: 4px; }
.callout-warning .callout-title { color: #d29922; }
.callout-important .callout-title { color: #bc8cff; }
table { border-collapse: collapse; width: 100%; margin: 12px 0; font-size: 13px; }
th, td { border: 1px solid var(--border); padding: 6px 10px; text-align: left; }
th { background: #161b22; font-weight: 600; color: #58a6ff; }
.link { color: var(--link); text-decoration: underline; }
"""
    },
    "2col": {
        "name": "Compact 2-Column Cheatsheet",
        "description": "High-contrast dense cheatsheet, 1200px width, 11px font, 2-column layout",
        "width": 1200,
        "css": """
:root {
  --bg: #121214;
  --fg: #e4e4e7;
  --heading: #38bdf8;
  --subheading: #a78bfa;
  --border: #27272a;
  --code-bg: #18181b;
  --code-fg: #34d399;
  --callout-bg: #1c1917;
  --font-base: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "JetBrains Mono", Consolas, Menlo, monospace;
}
body {
  margin: 0;
  padding: 20px 24px;
  background-color: var(--bg);
  color: var(--fg);
  font-family: var(--font-base);
  font-size: 11px;
  line-height: 1.42;
}
.container {
  column-count: 2;
  column-gap: 20px;
  column-rule: 1px solid var(--border);
}
h1 { font-size: 17px; color: var(--heading); border-bottom: 2px solid var(--heading); padding-bottom: 4px; margin: 0 0 10px 0; break-after: avoid; }
h2 { font-size: 14px; color: var(--heading); border-bottom: 1px solid var(--border); padding-bottom: 3px; margin: 12px 0 6px 0; break-after: avoid; }
h3 { font-size: 12px; color: var(--subheading); margin: 8px 0 4px 0; break-after: avoid; }
h4, h5, h6 { font-size: 11px; color: #cbd5e1; margin: 6px 0 2px 0; break-after: avoid; }
p { margin: 4px 0; }
ul, ol { padding-left: 18px; margin: 4px 0; }
li { margin: 2px 0; }
code {
  font-family: var(--font-mono);
  font-size: 10px;
  background: var(--code-bg);
  color: var(--code-fg);
  padding: 1px 4px;
  border-radius: 3px;
  border: 1px solid #3f3f46;
}
pre {
  background: var(--code-bg);
  border: 1px solid #3f3f46;
  border-radius: 4px;
  padding: 6px 8px;
  overflow-x: hidden;
  font-family: var(--font-mono);
  font-size: 10px;
  line-height: 1.35;
  margin: 6px 0;
  break-inside: avoid;
  white-space: pre-wrap;
  word-break: break-all;
}
pre code { background: none; border: none; padding: 0; color: #f4f4f5; }
blockquote {
  border-left: 3px solid #64748b;
  margin: 6px 0;
  padding: 2px 8px;
  color: #94a3b8;
  font-style: italic;
  break-inside: avoid;
}
hr { border: 0; border-top: 1px solid var(--border); margin: 10px 0; }
.callout {
  border-left: 3px solid var(--heading);
  background: #1e293b;
  padding: 6px 8px;
  margin: 6px 0;
  border-radius: 3px;
  break-inside: avoid;
}
.callout-warning, .callout-caution { border-left-color: #f59e0b; background: #2d2013; }
.callout-important { border-left-color: #ec4899; background: #2c1624; }
.callout-title { font-weight: bold; font-size: 9.5px; text-transform: uppercase; color: var(--heading); margin-bottom: 2px; }
.callout-warning .callout-title { color: #f59e0b; }
.callout-important .callout-title { color: #ec4899; }
table { border-collapse: collapse; width: 100%; margin: 6px 0; font-size: 10px; break-inside: avoid; }
th, td { border: 1px solid var(--border); padding: 4px 6px; text-align: left; }
th { background: #18181b; font-weight: 600; color: var(--heading); }
.link { color: #38bdf8; text-decoration: underline; }
"""
    },
    "3col": {
        "name": "Micro-Grid 3-Column Monospace",
        "description": "Ultra-dense monospace grid, 1400px width, 10px font, 3-column layout",
        "width": 1400,
        "css": """
:root {
  --bg: #09090b;
  --fg: #f4f4f5;
  --heading: #22d3ee;
  --subheading: #f472b6;
  --border: #27272a;
  --code-bg: #141417;
  --code-fg: #4ade80;
  --font-mono: "JetBrains Mono", "Fira Code", "Liberation Mono", Consolas, monospace;
}
body {
  margin: 0;
  padding: 16px 20px;
  background-color: var(--bg);
  color: var(--fg);
  font-family: var(--font-mono);
  font-size: 9.5px;
  line-height: 1.32;
}
.container {
  column-count: 3;
  column-gap: 16px;
  column-rule: 1px dashed var(--border);
}
h1 { font-size: 15px; color: var(--heading); border-bottom: 2px solid var(--heading); padding-bottom: 2px; margin: 0 0 8px 0; break-after: avoid; font-weight: bold; }
h2 { font-size: 12.5px; color: var(--heading); border-bottom: 1px solid var(--border); padding-bottom: 2px; margin: 10px 0 4px 0; break-after: avoid; }
h3 { font-size: 11px; color: var(--subheading); margin: 6px 0 3px 0; break-after: avoid; }
h4, h5, h6 { font-size: 10px; color: #a1a1aa; margin: 4px 0 2px 0; break-after: avoid; }
p { margin: 3px 0; }
ul, ol { padding-left: 14px; margin: 3px 0; }
li { margin: 1px 0; }
code {
  font-family: var(--font-mono);
  font-size: 9px;
  background: var(--code-bg);
  color: var(--code-fg);
  padding: 0 3px;
  border-radius: 2px;
  border: 1px solid #3f3f46;
}
pre {
  background: var(--code-bg);
  border: 1px solid #3f3f46;
  border-radius: 3px;
  padding: 4px 6px;
  overflow-x: hidden;
  font-family: var(--font-mono);
  font-size: 9px;
  line-height: 1.25;
  margin: 4px 0;
  break-inside: avoid;
  white-space: pre-wrap;
  word-break: break-all;
}
pre code { background: none; border: none; padding: 0; color: #e4e4e7; }
blockquote {
  border-left: 2px solid #71717a;
  margin: 4px 0;
  padding: 2px 6px;
  color: #a1a1aa;
  break-inside: avoid;
}
hr { border: 0; border-top: 1px dashed var(--border); margin: 8px 0; }
.callout {
  border-left: 2px solid var(--heading);
  background: #111827;
  padding: 4px 6px;
  margin: 4px 0;
  border-radius: 2px;
  break-inside: avoid;
}
.callout-warning, .callout-caution { border-left-color: #fbbf24; background: #1c150b; }
.callout-important { border-left-color: #f472b6; background: #1e0e18; }
.callout-title { font-weight: bold; font-size: 8.5px; text-transform: uppercase; color: var(--heading); margin-bottom: 2px; }
.callout-warning .callout-title { color: #fbbf24; }
.callout-important .callout-title { color: #f472b6; }
table { border-collapse: collapse; width: 100%; margin: 4px 0; font-size: 9px; break-inside: avoid; }
th, td { border: 1px solid var(--border); padding: 2px 4px; text-align: left; }
th { background: #18181b; font-weight: 600; color: var(--heading); }
.link { color: #38bdf8; text-decoration: underline; }
"""
    }
}


def build_full_html(body_html: str, layout_key: str, title: str) -> str:
    layout = LAYOUT_STYLES[layout_key]
    css = layout["css"]
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{html.escape(title)}</title>
<style>
{css}
</style>
</head>
<body>
<div class="container">
{body_html}
</div>
</body>
</html>
"""


def compute_token_metrics(text: str, width: int, height: int):
    """
    Computes text and multimodal token counts.
    
    Formulas:
    - Raw Markdown: word_count * 1.33 (approx standard cl100k / o200k / claude tokenization)
    - Claude Vision: (Width * Height) / 750
    - OpenAI / Codex Vision: Base 85 + 170 * (ceil(W/512) * ceil(H/512)) tiles
    - Gemini Vision: ~258 tokens per tile (fixed image / tile grid)
    """
    words = len(text.split())
    chars = len(text)
    raw_tokens = int(math.ceil(words * 1.33))

    # Claude formula
    claude_tokens = int(math.ceil((width * height) / 750.0))

    # OpenAI / Codex high-res formula
    # First scale so that shortest side <= 768, max side <= 2048 (if larger)
    scale = 1.0
    if width > 2048 or height > 2048:
        scale = min(2048.0 / width, 2048.0 / height)
    scaled_w = width * scale
    scaled_h = height * scale
    if min(scaled_w, scaled_h) > 768:
        short_scale = 768.0 / min(scaled_w, scaled_h)
        scaled_w *= short_scale
        scaled_h *= short_scale

    tiles_x = math.ceil(scaled_w / 512.0)
    tiles_y = math.ceil(scaled_h / 512.0)
    total_tiles = tiles_x * tiles_y
    openai_tokens = int(85 + (170 * total_tiles))

    # Gemini 1.5 / 2.0 vision: 258 tokens per tile (1 tile per 384x384 / image grid)
    # Gemini uses 258 tokens for standard images, up to N*258 for multi-tile high-res
    gemini_tiles = math.ceil(width / 768.0) * math.ceil(height / 768.0)
    gemini_tokens = int(max(258, 258 * gemini_tiles))

    return {
        "words": words,
        "chars": chars,
        "raw_text_tokens": raw_tokens,
        "image_width": width,
        "image_height": height,
        "claude_vision_tokens": claude_tokens,
        "claude_compression_ratio": round(raw_tokens / max(1, claude_tokens), 2),
        "openai_vision_tokens": openai_tokens,
        "openai_compression_ratio": round(raw_tokens / max(1, openai_tokens), 2),
        "gemini_vision_tokens": gemini_tokens,
        "gemini_compression_ratio": round(raw_tokens / max(1, gemini_tokens), 2),
    }


def get_image_dimensions(image_path: Path):
    """Returns (width, height) using ImageMagick identify or file inspection."""
    res = subprocess.run(
        ["identify", "-format", "%w %h", str(image_path)],
        capture_output=True,
        text=True,
        check=True
    )
    w, h = res.stdout.strip().split()
    return int(w), int(h)


def render_document(
    input_file: Path,
    outdir: Path,
    layouts=("1col", "2col", "3col"),
    formats=("png", "jpg")
):
    outdir.mkdir(parents=True, exist_ok=True)
    raw_text = input_file.read_text(encoding="utf-8")
    doc_stem = input_file.stem
    body_html = parse_markdown_to_html(raw_text)

    results = []

    for layout_key in layouts:
        layout_cfg = LAYOUT_STYLES[layout_key]
        width = layout_cfg["width"]
        html_file = outdir / f"{doc_stem}_{layout_key}.html"
        full_html = build_full_html(body_html, layout_key, doc_stem)
        html_file.write_text(full_html, encoding="utf-8")

        # Initial large render to capture full content height
        raw_png = outdir / f"{doc_stem}_{layout_key}_raw.png"
        final_png = outdir / f"{doc_stem}_{layout_key}.png"
        final_jpg = outdir / f"{doc_stem}_{layout_key}.jpg"

        # Viewport height large enough to avoid cutoffs (e.g. 8000px)
        viewport_h = 9000

        # Run Chromium headless
        chrome_cmd = [
            "flatpak", "run",
            f"--filesystem={os.path.abspath(outdir.parent.parent)}",
            "org.chromium.Chromium",
            "--headless=new",
            "--no-sandbox",
            "--disable-gpu",
            "--hide-scrollbars",
            "--force-device-scale-factor=1",
            f"--window-size={width},{viewport_h}",
            f"--screenshot={raw_png.resolve()}",
            f"file://{html_file.resolve()}"
        ]
        
        proc = subprocess.run(chrome_cmd, capture_output=True, text=True)
        if proc.returncode != 0:
            print(f"Error running Chromium: {proc.stderr}", file=sys.stderr)
            continue

        if not raw_png.exists():
            print(f"Failed to generate {raw_png}", file=sys.stderr)
            continue

        # Trim bottom padding using ImageMagick and add a uniform 24px border
        subprocess.run(
            ["convert", str(raw_png), "-trim", "-bordercolor", "#0d1117" if layout_key == "1col" else ("#121214" if layout_key == "2col" else "#09090b"), "-border", "24x24", "+repage", str(final_png)],
            check=True
        )
        if raw_png.exists():
            raw_png.unlink()

        # Convert to high quality JPG
        if "jpg" in formats:
            subprocess.run(
                ["convert", str(final_png), "-quality", "92", str(final_jpg)],
                check=True
            )

        img_w, img_h = get_image_dimensions(final_png)
        png_size_bytes = final_png.stat().st_size
        jpg_size_bytes = final_jpg.stat().st_size if final_jpg.exists() else 0

        metrics = compute_token_metrics(raw_text, img_w, img_h)
        metrics["layout"] = layout_key
        metrics["layout_name"] = layout_cfg["name"]
        metrics["png_path"] = str(final_png)
        metrics["png_size_bytes"] = png_size_bytes
        metrics["jpg_path"] = str(final_jpg) if final_jpg.exists() else None
        metrics["jpg_size_bytes"] = jpg_size_bytes
        metrics["doc"] = input_file.name

        results.append(metrics)

    return results


def print_markdown_table(all_results):
    print("\n### Vision vs Raw Markdown Token Efficiency Benchmark\n")
    print("| Document | Layout | Dimensions (WxH) | PNG Size | Raw Text Tokens | Claude Vision | Claude Ratio | OpenAI Vision | OpenAI Ratio | Gemini Vision | Gemini Ratio |")
    print("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |")
    for r in all_results:
        print(
            f"| `{r['doc']}` | {r['layout']} ({r['layout_name']}) | {r['image_width']}×{r['image_height']} | "
            f"{r['png_size_bytes'] // 1024} KB | {r['raw_text_tokens']:,} | "
            f"{r['claude_vision_tokens']:,} | **{r['claude_compression_ratio']}x** | "
            f"{r['openai_vision_tokens']:,} | **{r['openai_compression_ratio']}x** | "
            f"{r['gemini_vision_tokens']:,} | **{r['gemini_compression_ratio']}x** |"
        )
    print("")


def main():
    parser = argparse.ArgumentParser(description="Render Markdown to screenshot images and benchmark token efficiency.")
    parser.add_argument("inputs", nargs="+", help="Input markdown file(s)")
    parser.add_argument("--outdir", default="scratch/vision", help="Output directory for screenshots")
    parser.add_argument("--layouts", nargs="+", default=["1col", "2col", "3col"], choices=["1col", "2col", "3col"])
    parser.add_argument("--formats", nargs="+", default=["png", "jpg"], choices=["png", "jpg"])
    parser.add_argument("--json", action="store_true", help="Print JSON output")
    args = parser.parse_args()

    outdir = Path(args.outdir)
    all_results = []

    for input_str in args.inputs:
        input_path = Path(input_str)
        if not input_path.exists():
            print(f"Error: file not found: {input_path}", file=sys.stderr)
            continue
        print(f"Rendering {input_path} across layouts {args.layouts}...")
        res = render_document(input_path, outdir, layouts=args.layouts, formats=args.formats)
        all_results.extend(res)

    if args.json:
        print(json.dumps(all_results, indent=2))
    else:
        print_markdown_table(all_results)

    # Save summary json to outdir
    summary_file = outdir / "benchmark_summary.json"
    summary_file.write_text(json.dumps(all_results, indent=2), encoding="utf-8")
    print(f"Saved benchmark summary to {summary_file}")


if __name__ == "__main__":
    main()
