#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later

import os
import subprocess
from pathlib import Path

OUT_DIR = Path("/home/uwe/projects/harnez/scratch/vision")
OUT_DIR.mkdir(parents=True, exist_ok=True)

HTML_PATH = OUT_DIR / "font_probe.html"
PNG_PATH = OUT_DIR / "font_probe.png"

FONT_SIZES = [6, 7, 8, 9, 10, 11, 12]

html_sections = []
for sz in FONT_SIZES:
    html_sections.append(f"""
    <div class="probe-block font-{sz}">
      <div class="label">--- PROBE FONT SIZE: {sz}px ---</div>
      <div class="code-line"><span class="token">TokenTest[{sz}]:</span> if test "$x" != "$y" &amp;&amp; test -f "${{PATH:?}}"; then local val=$(cmd); fi</div>
      <div class="code-line"><span class="token">GlyphTest[{sz}]:</span> {{ [ ( &lt; &gt; == != := &amp;&amp; || ! ~ * ; : / \\ # @ $ % ^ ) ] }}</div>
      <div class="code-line"><span class="token">TextTest[{sz}]:</span> Invariant 3: Zero Zombie Guarantee (exit=127, pid=9482, score=5.0)</div>
    </div>
    """)

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
  padding: 24px;
  width: 1000px;
}}
h1 {{
  color: #58a6ff;
  font-size: 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid #30363d;
  padding-bottom: 8px;
}}
.probe-block {{
  margin-bottom: 14px;
  padding: 10px 14px;
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 6px;
}}
.label {{
  color: #79c0ff;
  font-weight: bold;
  margin-bottom: 6px;
}}
.code-line {{
  color: #e6edf3;
  line-height: 1.4;
  margin-bottom: 3px;
}}
.token {{
  color: #ff7b72;
  font-weight: bold;
}}
{" ".join([f".font-{sz} {{ font-size: {sz}px; }}" for sz in FONT_SIZES])}
</style>
</head>
<body>
<h1>Vision Transformer (ViT) Font Resolution &amp; OCR Breakdown Probe</h1>
{"".join(html_sections)}
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
    "--window-size=1020,950",
    f"file://{HTML_PATH}"
]
subprocess.run(cmd, check=True)
print(f"Generated stress-card screenshot at {PNG_PATH}")
