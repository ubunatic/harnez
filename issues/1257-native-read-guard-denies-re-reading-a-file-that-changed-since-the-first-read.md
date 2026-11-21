# 1257 — Native Read guard denies re-reading a file that changed since the first read

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1240, 1079

---

## 1. Problem & Motivation
Reported 2026-11-21 from the webman session (harnez 0.5.10+5413, Claude Code). The agent read a background task's output file (`<claude tmp>/tasks/<id>.output`, outside the repo) while the task was still running. Claude Code reported it as existing but empty. When the task finished, a second native Read of the same file was denied:

`harnez guard: this session already read this file natively, so a second native Read of it is denied even when the slice is small. Use the harnez read command below ...`

The first read put no content in context, so the guard's reason (avoiding re-reads of content already in context) does not apply. It cost one extra call. Background task output files are written to after they are first read, so this will recur.

## 2. Technical Specification / Findings
- 1240 covers re-reads after compaction (markers cleared by `compact-reset`). This case had no compaction between the two reads.
- Unverified: whether the native-read marker stores the file's size or mtime.

## 3. Implementation & Verification Plan
/goal A native Read of a file whose size or mtime changed since the session's last native Read of it is allowed. Stop and report if the hook has no cheap way to store and compare that state.

- Store size and mtime with the read marker, and allow the Read when either differs (or when the first read returned an empty file).
- Test: read a file, append to it, read again → allowed. Read again without a change → denied as today.
