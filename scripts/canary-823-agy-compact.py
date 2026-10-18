#!/usr/bin/env python3
"""Two-turn agy print-mode probe; uses only Gemini 3.8 Flash at low effort."""

import json
import pathlib
import shlex
import sqlite3
import subprocess
import tempfile


def snapshot(conversation, target):
    source = pathlib.Path.home() / ".gemini/antigravity-cli/conversations" / f"{conversation}.db"
    with sqlite3.connect(f"file:{source}?mode=ro", uri=True) as src:
        with sqlite3.connect(target) as dst:
            src.backup(dst)


def run(scratch, prompt, conversation=None):
    args = ["agy"]
    if conversation:
        args += ["--conversation", conversation]
    args += ["--add-dir", str(scratch), "--model", "gemini-3.8-flash",
             "--effort", "low", "--output-format", "json", "-p", prompt]
    result = subprocess.run(args, cwd=scratch, capture_output=True, text=True, timeout=120)
    record = {"command": shlex.join(args), "cwd": str(scratch), "exit": result.returncode,
              "stdout": result.stdout, "stderr": result.stderr}
    print(json.dumps(record, indent=2), flush=True)
    name = "compact.json" if conversation else "turn1.json"
    (scratch / name).write_text(json.dumps(record, indent=2))
    result.check_returncode()
    data = json.loads(result.stdout)
    if data["status"] != "SUCCESS":
        raise RuntimeError(data)
    return data


def main():
    scratch = pathlib.Path(tempfile.mkdtemp(prefix="harnez-823-"))
    print(f"Artifacts: {scratch}", flush=True)
    first = run(scratch, "Remember canary marker amber-823. Reply only OK. Do not use tools.")
    conversation = first["conversation_id"]
    snapshot(conversation, scratch / "before-compact.db")
    run(scratch, "/compact", conversation)
    snapshot(conversation, scratch / "after-compact.db")
    for name in ["before-compact.db", "after-compact.db"]:
        with sqlite3.connect(scratch / name) as db:
            steps = db.execute("select idx, step_type, step_payload from steps").fetchall()
        summaries = [(idx, kind) for idx, kind, payload in steps if b"<summary>" in (payload or b"")]
        print(f"{name}: {len(steps)} steps, summary steps {summaries}")
    print("Context reduction: unavailable; JSON usage is not current context size.")


if __name__ == "__main__":
    main()
