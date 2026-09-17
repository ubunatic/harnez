#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later

import subprocess
from pathlib import Path

REPO_ROOT = Path("/home/uwe/projects/harnez")
OUT_DIR = REPO_ROOT / "scratch/vision"
OUT_DIR.mkdir(parents=True, exist_ok=True)

HTML_PATH = OUT_DIR / "dev_cheatsheet_3in1.html"
PNG_PATH = OUT_DIR / "dev_cheatsheet_3in1.png"

# Read 3 target docs
bash_doc = (REPO_ROOT / "docs/lang/Bash.md").read_text(encoding="utf-8")
make_doc = (REPO_ROOT / "docs/lang/Make.md").read_text(encoding="utf-8")
git_doc = (REPO_ROOT / "docs/lang/Git.md").read_text(encoding="utf-8")

def md_to_html_simple(text: str, title: str) -> str:
    lines = text.split("\n")
    out = [f'<div class="doc-column"><div class="doc-header">{title}</div>']
    in_code = False
    for line in lines:
        if line.startswith("```"):
            if in_code:
                out.append("</code></pre>")
                in_code = False
            else:
                lang = line[3:].strip()
                out.append(f'<pre class="code-{lang}"><code>')
                in_code = True
            continue
        if in_code:
            escaped = line.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
            out.append(f"{escaped}\n")
            continue
        if line.startswith("# "):
            out.append(f'<h1 class="h1">{line[2:]}</h1>')
        elif line.startswith("## "):
            out.append(f'<h2 class="h2">{line[3:]}</h2>')
        elif line.startswith("### "):
            out.append(f'<h3 class="h3">{line[4:]}</h3>')
        elif line.startswith("- ") or line.startswith("* "):
            out.append(f'<li class="li">{line[2:]}</li>')
        elif line.strip() == "":
            pass
        else:
            out.append(f'<p class="p">{line}</p>')
    if in_code:
        out.append("</code></pre>")
    out.append("</div>")
    return "\n".join(out)

col1 = md_to_html_simple(bash_doc, "Bash Rules")
col2 = md_to_html_simple(make_doc, "Make Rules")
col3 = md_to_html_simple(git_doc, "Git Rules")

html_content = f"""<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
* {{ box-sizing: border-box; margin: 0; padding: 0; }}
body {{
  background: #0d1117;
  color: #c9d1d9;
  font-family: 'JetBrains Mono', 'Fira Code', 'DejaVu Sans Mono', monospace;
  font-size: 9.5px;
  line-height: 1.35;
  padding: 16px;
  width: 1420px;
}}
.title-banner {{
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  padding: 8px 14px;
  margin-bottom: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}}
.title-banner h1 {{
  color: #58a6ff;
  font-size: 13px;
  font-weight: bold;
}}
.badge {{
  background: #238636;
  color: #ffffff;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 9px;
}}
.grid-container {{
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 12px;
  align-items: start;
}}
.doc-column {{
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
  padding: 12px;
}}
.doc-header {{
  background: #21262d;
  color: #79c0ff;
  font-weight: bold;
  font-size: 11px;
  padding: 4px 8px;
  margin: -12px -12px 10px -12px;
  border-bottom: 1px solid #30363d;
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
}}
.h1 {{ color: #79c0ff; font-size: 11px; margin: 8px 0 4px 0; }}
.h2 {{ color: #58a6ff; font-size: 10px; margin: 6px 0 3px 0; border-bottom: 1px solid #21262d; padding-bottom: 2px; }}
.h3 {{ color: #d2a8ff; font-size: 9.5px; margin: 4px 0 2px 0; }}
.p {{ margin-bottom: 4px; }}
.li {{ margin-left: 12px; margin-bottom: 2px; list-style-type: square; }}
pre {{
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 4px;
  padding: 6px;
  margin: 4px 0 6px 0;
  overflow: hidden;
}}
code {{
  font-family: inherit;
  font-size: 9px;
  color: #7ee787;
}}
</style>
</head>
<body>
<div class="title-banner">
  <h1>Harnez Core Developer Cheatsheet (3-in-1 Bounded Card)</h1>
  <span class="badge">~1,100 Vision Tokens vs ~6,500 Text Tokens (6x Compression)</span>
</div>
<div class="grid-container">
  {col1}
  {col2}
  {col3}
</div>
</body>
</html>
"""

HTML_PATH.write_text(html_content, encoding="utf-8")
print(f"Wrote HTML to {HTML_PATH}")

cmd = [
    "flatpak", "run",
    "--filesystem=/home/uwe/projects/harnez",
    "org.chromium.Chromium",
    "--headless",
    "--disable-gpu",
    f"--screenshot={PNG_PATH}",
    "--window-size=1440,1400",
    f"file://{HTML_PATH}"
]
subprocess.run(cmd, check=True)
print(f"Generated 3-in-1 bounded cheatsheet at {PNG_PATH}")
