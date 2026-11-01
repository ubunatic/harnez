## Target: internal/subagent/jules.go

1. `internal/subagent/jules.go:610`
   - **Proposal:** Ensure transient API errors are matched before `default` in the switch block in `track`. When `apiErr.transient()` is true, the `case errors.As(err, &apiErr) && !apiErr.transient():` branch is skipped, falling through to `default:`, which increments the `failures` counter and checks against `spec.MaxPollErrors`. If transient errors are expected to not consume the `MaxPollErrors` budget, there should be an explicit case for `errors.As(err, &apiErr) && apiErr.transient():` that logs or continues without incrementing `failures`.
   - **Smallest next step:** Add a case inside the `track()` switch block to match transient errors and reset or ignore failures.

2. `internal/subagent/jules.go:179`
   - **Proposal:** The code reads from the response body up to `julesMaxResponseBytes` but doesn't check if the limit was actually hit. When reading with `io.LimitReader(resp.Body, julesMaxResponseBytes)`, it doesn't return an error if it truncates the payload (it just silently returns EOF after `julesMaxResponseBytes`). Consequently, `json.Unmarshal(data, out)` on line 188 will fail with a generic JSON decode error if a valid JSON was truncated.
   - **Smallest next step:** Check if `len(data) == julesMaxResponseBytes` and if so, return a clear error about the response being too large before attempting JSON unmarshal.

3. `internal/subagent/jules.go:375`
   - **Proposal:** In `branchDriftWarning`, it loops over all remotes sequentially doing `ls-remote` (via `head(remote)`), which can be very slow if there are many remotes and no concurrency is used.
   - **Smallest next step:** Run `head(remote)` concurrently for all remotes using a WaitGroup to significantly speed up `branchDriftWarning`.

4. `internal/subagent/jules.go:429`
   - **Proposal:** In `syncGitHubBranch`, when checking if objects exist locally before fetching, the code uses `d.git(ctx, "cat-file", "-e", source.sha+"^{commit}")`. If it fails, it falls back to fetching `source.remote, ref` (which is `refs/heads/branch`). By the time we fetch the `ref` (branch), the remote branch might have moved forward to a different commit, and we still won't have `source.sha` locally, which will then cause `merge-base` in the next step to fail because `source.sha` is missing.
   - **Smallest next step:** Update the `fetch` command to also explicitly try fetching the `source.sha` directly, or verify `source.sha` exists *after* fetching `ref`.

5. `internal/subagent/jules.go:230`
   - **Proposal:** The code constructs `exec.CommandContext` using `append([]string{"-C", d.Dir}, args...)`. If `d.Dir` is empty, this ends up being `git -C ""`, which will cause `git` to error out. While `d.Dir` typically defaults to `.` or is populated, there's no check preventing it from being empty. This would break all `git` commands in `JulesDriver`.
   - **Smallest next step:** Initialize `d.Dir = "."` if it is empty before appending to `-C` arguments.
