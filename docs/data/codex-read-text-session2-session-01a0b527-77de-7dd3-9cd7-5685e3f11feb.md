# Codex conversation

## User

read Makefile

## Assistant

I’ll inspect the repository’s local overlay and Makefile, then summarize the available targets and conventions.

## Activity

    $ /usr/bin/zsh -lc "⚙ bash -c 'sed -n '\\''1,240p'\\'' AGENTS.local.md 2>/dev/null; printf '\\''\\n--- Makefile ---\\n'\\''; sed -n '\\''1,260p'\\'' Makefile'"
    status: Completed · exit 0
      <!-- harnez:begin Concise Mode -->
      ## Operational Mode: Concise Lite (Level 1)
      - Drop opening pleasantries and speculative closing remarks.
      - Keep full grammar and complete-sentence explanations.
      - Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.
      - Reference: @docs/practices/ConciseMode.md
      <!-- harnez:end Concise Mode -->
      
      ## Local Correction: GitHub Mirror Sync
      - `docs/Git.md`/`docs/lang/Git.md` describe the GitHub mirror as auto-synced via
        Codeberg's platform push-mirroring — that is **not** actually configured for this
        repo's `github.com/ubunatic/harnez` remote.
      - Until platform mirroring is set up (or this note is removed), push explicitly to
        both remotes when the GitHub mirror needs to be current: `git push origin main &&
        git push github main`.
      
      --- Makefile ---
      .PHONY: ⚙️ 🤖  # ⚙️ = manual/once, 🤖 = managed
      
      _prim := \033[36m
      _rst  := \033[0m
      
      BINARY   ?= harnez
      CONFIG   := config.yaml
      TARGET   := $(HOME)/.claude
      PROJECT  := .
      LANGS    := golang bash make git
      PREFIX   ?= /usr/local
      HOST     ?= um760
      HOST_DIR ?= projects/harnez
      
      help: 🤖  # show this help
      	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
      	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'
      
      preflight: ⚙️  # check toolchains and dependencies
      	@command -v go >/dev/null || (echo "❌ go is not installed" && exit 1)
      
      build: ⚙️  # build the binary
      	go build -o $(BINARY) ./cmd/harnez
      
      run: ⚙️ build  # run the application locally
      	./$(BINARY)
      
      install: ⚙️ build  # install binary to ~/go/bin (user)
      	go install ./cmd/harnez
      	@ln -sf harnez $$(go env GOPATH)/bin/⚙ 2>/dev/null || ln -sf harnez $(HOME)/go/bin/⚙
      
      install-tools: ⚙️  # install scripts/*/main.go canary CLIs to ~/go/bin
      	go install ./scripts/canary-agenticloop-lite
      
      install-all: install install-tools ⚙️  # install app and  tools
      
      install-system: ⚙️ build  # install binary to PREFIX/bin via sudo (system-wide)
      	sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)
      	sudo ln -sf $(BINARY) $(PREFIX)/bin/⚙
      
      uninstall: ⚙️  # remove installed binary from system and user paths
      	rm -f $(shell which $(BINARY) 2>/dev/null) $(PREFIX)/bin/$(BINARY) $(PREFIX)/bin/⚙ $(HOME)/go/bin/⚙ $$(go env GOPATH)/bin/⚙ 2>/dev/null
      
      apply: ⚙️ build  # apply config to ~/.claude globally
      	./$(BINARY) apply -c $(CONFIG) -t $(TARGET)
      
      init: ⚙️ build  # initialize or update this project
      	./$(BINARY) init -c $(CONFIG) -d $(PROJECT) -y
      
      init-siblings: ⚙️ build  # run harnez init across all sibling projects
      	@for dir in $$(find .. -maxdepth 1 -mindepth 1 -type d | sort); do \
      		if test -d "$$dir/.git" || test -f "$$dir/AGENTS.md" || test -f "$$dir/CLAUDE.md"; then \
      			if test "$$dir" != "../archive" && test "$$dir" != "../videos"; then \
      				echo "=== Updating $$dir ==="; \
      				./$(BINARY) init -d "$$dir" -y || true; \
      			fi; \
      		fi; \
      	done
      
      diff: ⚙️ build  # show what apply would change in managed blocks
      	./$(BINARY) diff -c $(CONFIG) -t $(TARGET)
      
      clean: ⚙️ build  # remove managed blocks from the Claude Code config directory
      	./$(BINARY) clean -c $(CONFIG) -t $(TARGET)
      
      status: ⚙️ build  # show config summary and applied state
      	./$(BINARY) status -c $(CONFIG) -t $(TARGET)
      
      lint: ⚙️  # check docs/commands/*.md files are all registered in config.yaml
      	bash scripts/lint.sh
      
      agent-canary-build: ⚙️ build  # build the shared Pi/OpenCode canary container
      	bash scripts/agent-canary/run.sh build
      
      agent-canary-static: ⚙️ build  # run deterministic in-container canary checks
      	bash scripts/agent-canary/run.sh static
      
      install-canary: ⚙️  # verify the latest Codeberg release installs and reports its version
      	bash scripts/install-canary.sh
      
      macos-ci: ⚙️  # trigger and watch macos-hello CI workflow on the GitHub mirror
      	bash scripts/macos-ci.sh
      
      check: ⚙️  # run static analysis and tests (GOWORK=off: catch go.mod pin drift behind a local workspace override)
      	GOWORK=off go vet ./...
      	GOWORK=off go test ./...
      
      check-fast: ⚙️  # fast local feedback loop
      	go test ./...
      
      smoke: ⚙️ build  # live smoke: apply/diff/repair + usage bar-alignment against real binary
      	bash scripts/smoke-test.sh
      
      test: ⚙️ check  # alias for check
      
      format: ⚙️  # format source code
      	go fmt ./...
      
      release: check ⚙️  # release the project using harnez
      	./$(BINARY) release
      
      sync: ⚙️  # push here, pull there, build there, verify
      	git push
      	ssh $(HOST) "cd $(HOST_DIR) && git pull && make install && make status"
      
      test-q1: 🤖  # run tests under Quota-1 enforcement
      	harnez exec --quota-1 -- $(MAKE) test

## Assistant

Makefile read.

Key targets:

- `build` — builds `harnez`
- `install` — installs to `~/go/bin`
- `apply`, `init`, `diff`, `clean`, `status` — manage configuration
- `check` — runs `go vet` and tests
- `test-q1` — quota-enforced test entry point
- `smoke` — runs the live smoke test
- `format`, `lint` — formatting and documentation checks
- `release`, `sync` — release and remote synchronization workflows

## Activity

    $ /usr/bin/zsh -lc 'harnez stats --auto'
    status: Completed · exit 0
      TOOL  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      Bash  1      n/a        16137.0     -               -              0.0%
      sed   1      5.00       n/a         -               -              0.0%
      
      AGENT  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      codex  2      5.00       16137.0     -               -              0.0%
      
      PROJECT  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      harnez   2      5.00       16137.0     -               -              0.0%
      
      compaction economics: insufficient_data (0 compaction(s))
        note: no compaction economics recorded
      distillation byte savings: no rows with distillation data
      
      heartbeat (harnez rate --ok, issue 179): none recorded

## User

read each doc in docs/lang and summarise each doc as a bullet point list with one short sentence per doc

## Assistant

I’ll enumerate `docs/lang/` and read each document’s contents, then provide one concise summary sentence per file.

## Activity

    $ /usr/bin/zsh -lc "⚙ bash -c 'rg --files docs/lang | sort'"
    status: Completed · exit 0
      docs/lang/Bash.lite.md
      docs/lang/Bash.md
      docs/lang/Cpp.md
      docs/lang/GTK4.md
      docs/lang/Git.md
      docs/lang/Go.md
      docs/lang/Make.lite.md
      docs/lang/Make.md
      docs/lang/ManPages.md
      docs/lang/Markdown.md
      docs/lang/Rust.md
      docs/lang/Zig.md

## Activity

    $ /usr/bin/zsh -lc "⚙ bash -c 'wc -l docs/lang/*; for f in docs/lang/*; do printf '\\''\\n===== %s =====\\n'\\'' \""'$f"; sed -n '"'\\''1,220p'\\'' \""'$f"; done'"'"
    status: Completed · exit 0
        158 docs/lang/Bash.lite.md
        242 docs/lang/Bash.md
         35 docs/lang/Cpp.md
         76 docs/lang/GTK4.md
         43 docs/lang/Git.md
        107 docs/lang/Go.md
        118 docs/lang/Make.lite.md
        148 docs/lang/Make.md
        207 docs/lang/ManPages.md
         65 docs/lang/Markdown.md
         39 docs/lang/Rust.md
        111 docs/lang/Zig.md
       1349 total
      
      ===== docs/lang/Bash.lite.md =====
      <!-- harnez:variant=lite -->
      # Bash Rules (Lite)
      
      ## 1. Header
      
      ```bash
      #!/usr/bin/env bash
      set -euo pipefail   # -e exit on non-zero; -u exit on unset var; -o pipefail last non-zero wins
      ```
      
      ## 2. Sourcing
      
      DO `source f` — readable, greppable (`grep 'source '`), distinct from `./script.sh`.
      DON'T `. f` — dot is lost in whitespace, confusable with paths, ungreppable.
      
      ```bash
      source ~/.bashrc   # ✅
      . ~/.bashrc        # ❌
      ```
      
      ## 3. Conditionals — TOP RULE: `if test`, never brackets
      
      ```bash
      if test -f "$file"                    # ✅ 3-line if-then-fi
      then printf 'Found %s\n' "$file"
      fi
      
      if test "$a" = "$b"                   # ✅ 4-line if-then-else-fi
      then printf 'Equal\n'
      else printf 'Not equal\n'
      fi
      
      if test -d "$dir"                     # ✅ multi-cmd: 1st cmd on `then` line, rest aligned under it
      then printf 'Entering %s\n' "$dir"
           process_dir "$dir"
      fi
      
      while test "$x" != "$y"               # ✅ 1st cmd on `do` line
      do process "$x"
      done
      ```
      
      DON'T (all forbidden, no exceptions, forget legacy usage):
      
      - `[ x = y ]` single brackets
      - `[[ x == y ]]` double brackets
      - `; then` / `; do` — break the line instead
      - `then` alone on its line (dangling) — 1st cmd goes on the `then` line
      - `test x = y && do_a || do_b` logic ops as control flow – use 4-line `if-then-else-fi`
      
      ## 4. Variables
      
      ```bash
      "$var" "${var}"                            # quote EVERY expansion
      pattern="${1:?Usage: script.sh PATTERN}"   # required arg
      f() {
         local name="$1"        # literal -> assign inline
         local result           # cmd substitution -> declare FIRST,
         result=$(cmd args)     # then assign; else `local` masks cmd exit code
      }
      ```
      
      ## 5. Output
      
      ```bash
      printf '%s\n' "$var"              # printf > echo for variables
      printf 'ERROR: %s\n' "$msg" >&2   # errors -> stderr
      
      pass() {
         printf '  ✓ %s\n' "$*"
      }
      
      fail() {
         printf 'ERROR: %s\n' "$*" >&2
         exit 1
      }
      ```
      
      ## 6. Continuation — align, never fixed indent
      
      ```bash
      result=$(some_command |        # pipeline: break after `|`, align under chain start
               grep "pattern" |
               awk '{print $2}')
      
      if test -f "$a" &&             # condition: break after `&&`/`||`, align under 1st test
         test -d "$b"
      then stmt1
           stmt2
      else fail "not found"
      fi
      
      for item in "${array[@]}"      # then/else/do: 1st cmd on keyword line, rest aligned
      do process_item "$item" || fail "err"
         log_item "$item"
      done
      ```
      
      Function body: base indent 3 inside `{ }`. Alignment continuation > fixed indent for conditionals.
      
      ## 7. Commands & traps
      
      ```bash
      command -v tool   # not `which`
      out=$(cmd 2>&1)   # capture
      cmd > f           # write
      cmd >> f          # append
      cmd 2>/dev/null   # drop errors
      tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT   # always trap-clean temps
      
      timeout 30 curl -sf https://example.com/health
      # wrap any uncertain-duration cmd lacking its own client-side timeout (net probe,
      # lock wait, external call). Size seconds per command, not one global value.
      # timeout exits 124 on kill -> branch on it separately from the cmd's own codes.
      # Per-invocation only; background jobs -> AgenticLoop.md "Blocking sleep Waits" /
      # "Buffered Long-Running Output".
      ```
      
      ## 8. Functions
      
      Define before first use. Status via `return 0`/`return 1` — never printed booleans.
      
      ## 9. Directory scoping — `-C` over `cd`
      
      Rule: command has a directory flag -> use it; never `cd` just for scoping. cwd persists
      across tool calls, so a stray `cd` silently retargets the *next* unrelated call (e.g.
      `git status` on the wrong repo) with no error.
      
      ```bash
      git -C DIR status                  # git    -C DIR
      make -C DIR test                   # make   -C DIR
      go -C DIR build ./...              # go     -C DIR (1.20+)
      npm --prefix DIR install           # npm    --prefix DIR
      cargo build --manifest-path DIR/Cargo.toml   # cargo --manifest-path FILE
      
      (cd DIR && some-tool --flag)   # ✅ no flag exists -> subshell, cwd auto-restored
      cd DIR && some-tool --flag     # ⚠️ leaks cwd through rest of THIS call only; keep in ONE call
                                     # ❌ bare `cd` meant to carry into a LATER call
      ```
      
      Shared-shell caveat: where the shell is shared with the user's interactive terminal, an
      agent's `cd` outlives the call and moves the human's prompt too. Advisory, not enforceable
      (issue 095) — the shell reaches anywhere the OS user can. Prefer `-C`/absolute paths/subshell;
      if a bare `cd` is unavoidable (tool takes relative paths only), capture and restore:
      
      ```bash
      orig=$(pwd)
      cd DIR
      some-tool --relative-only-flag
      cd "$orig"
      ```
      
      See issue 095 (cwd leak, trust boundary), issue 222 (multi-repo wrong-repo failure).
      
      ## 10. Awk Portability (rarely needed)
      
      Default is mawk, not gawk — no 3-arg `match(str, /re/, arr)` (use `split()`/`sub()`/`gsub()`),
      no `strtonum()` (write a manual `h2d()`), no `gensub()` (use `sub()`/`gsub()` + a temp var).
      
      ===== docs/lang/Bash.md =====
      ---
      title: Bash Conventions
      weight: 61
      ---
      
      <!-- harnez:bundled -->
      # Bash Conventions
      
      > **Who this is for** — anyone writing a canary, a build script, or any glue in these repositories. Reference material: grep it, don't read it.
      >
      > **Read this if** — you are about to write conditionals, sourcing statements, or multi-line shell scripts.
      >
      > **Takeaways**
      > 1. `set -euo pipefail`, always, on line two.
      > 2. `if test …` — never `[ … ]`, never `[[ … ]]`, no `;`, 3-line `if-then-fi` (`then <cmd>` on same line).
           Never abuse `&&`/`||`, use 4-line `if-then-else-fi` (`else <cmd>` on same line).
      > 3. Always use `source`, never `.` for scripts and dotfiles (`~/.bashrc`, `~/.zshrc`).
      > 4. Quote every expansion; declare `local` separately for command substitutions.
      > 5. Writing awk? Default is mawk, not gawk — see the optional Appendix before using gawk extensions.
      
      ---
      
      ## 1. Header & Strict Mode
      ```bash
      #!/usr/bin/env bash
      set -euo pipefail
      ```
      - `-e`: Exit immediately if any command returns a non-zero status.
      - `-u`: Exit if an uninitialized variable is referenced.
      - `-o pipefail`: Ensure pipeline return codes reflect the last non-zero command in the chain.
      
      ## 2. Sourcing Scripts & Dotfiles — Always `source`, Never `.`
      
      Always use the explicit `source` keyword instead of the single dot (`.`) syntax:
      
      ```bash
      # ✅ DO: explicit, searchable, unambiguous
      source ~/.bashrc
      source ~/.zshrc
      source "$script_dir/lib.sh"
      
      # ❌ DON'T: ambiguous dot easily lost in whitespace or confused with path prefixes
      . ~/.bashrc
      . ~/.zshrc
      . "$script_dir/lib.sh"
      ```
      
      - **Readability**: `source` makes the intent obvious at a glance to human reviewers and AI agents.
      - **Searchability**: Grepping for `source ` reliably locates script inclusions; grepping for `.` produces vast noise.
      - **Disambiguation**: Distinguishes file sourcing from relative directory execution (such as `./script.sh`).
      
      ## 3. Conditionals — Always `if test`, Never `[[ ]]` or `[ ]`
      
      **This is the most important rule!**
      **NEVER** use `[ ... ]` or `[[ ... ]]` for conditionals. Forget all legacy usages!
      Use the clean, standard `test` **command** with 3-line `if-then-fi` and 4-line `if-then-else-fi` vertical alignment:
      
      ```bash
      # ✅ 3-line if-then-fi (then <1st cmd> on the same line, no semicolons)
      if test -f "$file"
      then printf 'Found %s\n' "$file"
      fi
      
      # ✅ 4-line if-then-else-fi
      if test "$a" = "$b"
      then printf 'Equal\n'
      else printf 'Not equal\n'
      fi
      
      # ✅ Multi-command then block (1st cmd on same line; subsequent cmds aligned)
      if test -d "$dir"
      then printf 'Entering %s\n' "$dir"
           process_dir "$dir"
      fi
      
      # ✅ while loop (do <1st cmd> on same line)
      while test "$x" != "$y"
      do process "$x"
      done
      ```
      
      ### Visual Do / Don't Anti-Patterns
      
      | Style | Pattern | Status | Rationale |
      |---|---|---|---|
      | **Harnez Standard (3-line)** | `if test "$x" = "$y"`<br>`then do_work`<br>`fi` | ✅ **DO** | Clean 3-line block, explicit command, no semicolon clutter. |
      | **Harnez Standard (4-line)** | `if test "$x" = "$y"`<br>`then do_work`<br>`else do_other`<br>`fi` | ✅ **DO** | Clean 4-line branch, first commands placed directly after `then`/`else`. |
      | **Harnez Sourcing** | `source ~/.bashrc`<br>`source "$lib"` | ✅ **DO** | Explicit `source` keyword for scripts and dotfiles. |
      | **Legacy Bracket** | `if [ "$x" = "$y" ]; then`<br>`  do_work`<br>`fi` | ❌ **DON'T** | Single brackets `[ ... ]` are forbidden. |
      | **Bash Extension** | `if [[ "$x" == "$y" ]]; then`<br>`  do_work`<br>`fi` | ❌ **DON'T** | Double brackets `[[ ... ]]` are forbidden. |
      | **Semicolon Suffix** | `if test "$x" = "$y"; then`<br>`  do_work`<br>`fi` | ❌ **DON'T** | Semicolons before `then`/`do` are forbidden; break lines instead. |
      | **Logic Operator Abuse** | `test "$x" = "$y" && do_a \|\| do_b` | ❌ **DON'T** | Logic operators are not control flow statements. |
      | **Dangling Then (POSIX)** | `if test "$x" = "$y"`<br>`then`<br>`  do_work`<br>`fi` | ❌ **DON'T** | Avoid empty `then` line; place 1st command directly after `then`. |
      | **Ambiguous Dot Sourcing** | `. ~/.bashrc`<br>`. "$lib"` | ❌ **DON'T** | Standalone `.` for sourcing is forbidden; use `source`. |
      
      ## 4. Variables & Local Scope
      - Always double-quote variable expansions: `"$var"`, `"${var}"`.
      - Handle required arguments with default error patterns:
        `pattern="${1:?Usage: script.sh PATTERN}"`
      - **Local variables in functions**:
        - Assign literal values directly: `local name="$1"`
        - **Command substitutions MUST declare first, then assign**, to prevent `local` from masking execution exit codes:
          ```bash
          local result
          result=$(command args)
          ```
      
      ## 5. Output Discipline
      - Prefer `printf` over `echo` for printing variables: `printf '%s\n' "$var"`.
      - Log errors to stderr: `printf 'ERROR: %s\n' "$msg" >&2`.
      - Status helpers used across repository scripts:
        ```bash
        pass() {
           printf '  ✓ %s\n' "$*"
        }
      
        fail() {
           printf 'ERROR: %s\n' "$*" >&2
           exit 1
        }
        ```
      
      ## 6. Line Breaks, Continuation & Indentation
      Avoid arbitrary fixed indentation for command blocks. Prefer **alignment continuation**:
      
      ### Continuation Rules
      - **Long pipelines**: break after `|`; line up under the start of the chain:
        ```bash
        result=$(some_command |
                 grep "pattern" |
                 awk '{print $2}')
        ```
      - **Long conditions**: break after `&&`/`||`; line up conditions under the first test:
        ```bash
        if test -f "$a" &&
           test -d "$b"
        then stmt1
             stmt2
        else fail "not found"
        fi
        ```
      - **Then/Else & Loop blocks**: 1st command directly after `then`/`else`/`do`; subsequent commands aligned:
        ```bash
        for item in "${array[@]}"
        do process_item "$item" || fail "err"
           log_item "$item"
        done
        ```
      - **Function bodies**: base indent level 3 inside `{ ... }`. Alignment continuation takes precedence over fixed indents for conditionals.
      
      ## 7. Commands & Traps
      - Prefer `command -v` over `which`.
      - Capture output cleanly: `out=$(cmd 2>&1)`.
      - Redirects: `> file` to write, `>> file` to append, `2>/dev/null` to suppress errors.
      - Temp files: `mktemp`; clean up with `trap 'rm -f "$tmp"' EXIT`.
      - **Prefix uncertain-duration commands with `timeout`**: when invoking something
        without a client-side timeout of its own — a network probe, a lock wait, an
        external service call — wrap it rather than risk an indefinite hang:
        ```bash
        timeout 30 curl -sf https://example.com/health
        ```
        Size the seconds argument to the command's expected duration, not one fixed
        global value. `timeout` exits `124` when it kills the command — check for
        that distinctly from the wrapped command's own failure exit codes if
        downstream logic branches on `$?`. This is about individual command
        invocations; for waiting on a long-running background job instead, see
        `docs/practices/AgenticLoop.md`'s "Blocking sleep Waits" and "Buffered
        Long-Running Output" anti-patterns.
      
      ## 8. Functions
      - Define functions before first invocation.
      - Return status with `return 0` / `return 1` (exit codes, never printed booleans).
      
      ## 9. Directory Scoping — Prefer `-C` Over `cd`
      
      **Rule**: if the command has a directory flag, use it — never `cd` purely for scoping.
      
      The shell tool's cwd persists across tool calls within a session. A `cd` in one call
      silently changes where the *next* unrelated call runs, producing wrong-repo results
      with no error (e.g. `git status` reporting on the wrong repo after a stray `cd`).
      
      ### Flag table (verified against each tool's help)
      
      | Command | Directory flag | Example |
      |---------|---------------|---------|
      | `git`   | `-C <dir>`    | `git -C ~/projects/foo status` |
      | `make`  | `-C <dir>`    | `make -C ~/projects/foo test` |
      | `go`    | `-C <dir>` (Go 1.20+) | `go -C ~/projects/foo build ./...` |
      | `npm`   | `--prefix <dir>` | `npm --prefix ~/projects/foo install` |
      | `cargo` | `--manifest-path <path>` | `cargo build --manifest-path ~/projects/foo/Cargo.toml` |
      
      ### Fallback — when no flag exists
      
      If the tool has no directory flag, keep the `cd` and the command in **one** call and
      prefer the subshell form so cwd is restored automatically even within that call:
      
      ```bash
      # ✅ Subshell: cwd is restored when the subshell exits
      (cd /some/dir && some-tool --flag)
      
      # ⚠️  Inline: cwd leaks into the rest of this call, but at least doesn't persist
      #    into the next tool call (do not split across calls)
      cd /some/dir && some-tool --flag
      ```
      
      Never issue a bare `cd` with the intent of letting its effect carry into a *later*
      separate tool call — that is the failure mode.
      
      ### Restore-cwd convention for shared shell environments
      
      In environments where the shell tool's session is **shared with the user's interactive
      terminal** (not an isolated subprocess per call), a `cd` the agent issues outlives the
      tool call and silently changes the *human's* prompt too.
      
      **Advisory mitigation** (this cannot be mechanically enforced — see issue 095):
      
      - Prefer `git -C`/`make -C`, absolute paths, or the subshell form `(cd dir && cmd)`
        as the first choice; they restore cwd automatically.
      - If a bare `cd` is unavoidable (tool only accepts relative paths), `cd` back to the
        starting directory before the tool call ends. Capture the start first:
      
      ===== docs/lang/Cpp.md =====
      ---
      title: C++ Conventions
      weight: 66
      ---
      
      <!-- harnez:bundled -->
      # C/C++ conventions
      
      ## Build acceleration
      
      ### ccache with CGO (Go + C/C++ via cgo)
      
      Set `CC` and `CXX` to the ccache wrappers when invoking `go build` for CGO targets:
      
      ```makefile
      CC  = ccache gcc
      CXX = ccache g++
      ```
      
      Export both in the build environment variable block, e.g.:
      
      ```makefile
      QT_BUILD_ENV := CC="ccache gcc" CXX="ccache g++" GOGC=50 GOMEMLIMIT=512MiB
      build-qt:
      	$(QT_BUILD_ENV) go build -p 1 -v -o instamoji-qt ./cmd/instamoji-qt
      ```
      
      Install ccache: `sudo apt install ccache`
      
      Check hit rate: `ccache -s`
      
      ### Notes
      
      - `-p 1` (single parallel job) is required for heavy Qt/CGO builds to avoid OOM on low-RAM hosts; ccache compensates for the lost parallelism on rebuilds.
      - `GOGC=50 GOMEMLIMIT=512MiB` caps Go GC pressure during the CGO compile phase.
      
      ===== docs/lang/GTK4.md =====
      ---
      title: GTK4 Conventions
      weight: 67
      ---
      
      <!-- harnez:bundled -->
      
      <!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
      <!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
      
      # GTK4 and PyGObject Development Guidelines
      
      This document details development guidelines, architectural patterns, and resolved pitfalls for PyGObject and GTK4 UI programming.
      
      ---
      
      ## 1. Application Uniqueness
      
      * **Standard**: Always use `Gio.ApplicationFlags.NON_UNIQUE` application flags.
      * **Context**: When an application is spawned from a file manager (like Nautilus) or a command-line script, a unique application instance flag would cause subsequent spawns to silently send files to the background daemon instead of opening a new CLI/GUI window. Using non-unique flags ensures each invocation gets its own independent process and window context.
      
      ---
      
      ## 2. Event Gesture Consolidation
      
      * **Standard**: Avoid placing both `Gtk.GestureClick` and `Gtk.GestureDrag` on the same drawing canvas or widget. Consolidate click and drag event handling inside a single `Gtk.GestureDrag` controller.
      * **Context**: GTK4 coordinates mouse press and release across gesture controllers. A click triggers a brief press state, firing `drag-begin` immediately followed by `drag-end` (with zero or near-zero displacement). If click reset logic runs alongside drag tracking, the release event clears the active crop box or overlay state, resulting in flickers where the drawing canvas appears and instantly vanishes.
      * **Implementation Pattern**:
        ```python
        # Setup the single drag gesture
        drag_gesture = Gtk.GestureDrag.new()
        drag_gesture.connect("drag-begin", self._on_drag_begin)
        drag_gesture.connect("drag-update", self._on_drag_update)
        drag_gesture.connect("drag-end", self._on_drag_end)
        self.widget.add_controller(drag_gesture)
        ```
        Distinguish simple clicks inside `_on_drag_end` by checking offset displacement:
        ```python
        def _on_drag_end(self, gesture, offset_x, offset_y):
            is_click = abs(offset_x) < 5.0 and abs(offset_y) < 5.0
            if is_click:
                # Handle simple click click-action
                self._on_click_detected()
            else:
                # Keep the dragged selection box
                pass
        ```
      
      ---
      
      ## 3. Cairo Overlay Drawing Areas
      
      * **Standard**:
        1. Set alignment properties of drawing area overlays to `FILL`.
        2. Use size parameters passed directly to the draw callback instead of calling `widget.get_width()`.
      * **Context**: `Gtk.Overlay` child widgets may not expand to cover the parent layout if alignments are unspecified. Additionally, calling `get_width()` / `get_height()` on widgets during layout stages might return incomplete layout info (`0` or `1`).
      * **Implementation Pattern**:
        ```python
        self.crop_overlay = Gtk.DrawingArea()
        self.crop_overlay.set_halign(Gtk.Align.FILL)
        self.crop_overlay.set_valign(Gtk.Align.FILL)
        self.crop_overlay.set_draw_func(self._on_crop_overlay_draw)
        ```
        Use `w` and `h` arguments in the draw function:
        ```python
        def _on_crop_overlay_draw(self, area, cr, w, h):
            # w and h represent the actual allocated screen pixels
            bounds = self._get_image_bounds(w, h)
        ```
      
      ---
      
      ## 4. Integration Smoke Testing
      
      * **Standard**: Always run integration smoke tests (`test_integration.py`) to verify window presentation behavior.
      * **Context**: Import-time and startup-time AttributeErrors or Gtk layout crashes will not be caught by static code analysis or mocks. Running the compiled executable inside a virtual framebuffer ensures the application launches successfully.
      
      ===== docs/lang/Git.md =====
      ---
      title: Git Conventions
      weight: 63
      ---
      
      <!-- harnez:bundled -->
      # Git conventions
      
      Applies unless overridden by project-local instructions for coding agents,
      commonly stored in an `AGENTS.md` file at the repository root.
      
      ## Workflow
      - Work local, work on the repo's default branch (usually `main`, sometimes `master`) —
        no local feature branches unless the project's Repo Setup says otherwise.
      - Commit proactively:
        - Default preference: commit autonomously after intermediate steps once tests are clean, after finished features (with review pass), between iteration attempts on a stuck bug, and immediately after filing issue-tracker entries.
        - Ask-first harness interaction: If running in a harness that mandates explicit user confirmation for `git commit`, ask the user **upfront before starting work / during session kickoff** for authorization to commit local checkpoints proactively. This enables the user to step away without returning to uncommitted progress.
        - Orchestrator delegation: When an orchestrating agent operates under an ask-first rule, it must clarify commit authority with the user at kickoff and specify whether spawned subagents commit directly or return diffs for the orchestrator to commit on their behalf.
      - Review before commit:
        - small fixes: commit directly if all tests pass and existing test assertions remain intact
        - non-trivial changes / features: perform a dedicated review pass (host orchestration or a fresh reviewer subagent) before committing to verify test rigor, doc/ticket accuracy, and codebase clarity
      - Do not push unless asked.
      - Do not create remote branches or PRs unless asked.
      
      ## Commit messages
      - Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary):
        `type(scope)?: summary`, e.g. `feat(cli): add init command`.
      - Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
      
      ## Hygiene
      - Do not commit secrets.
      - Do not commit large files unless tracked via git-lfs.
      - Do not commit PII.
      
      ## Remotes
      - Canonical origin: `codeberg.org/<username>/<reponame>`.
      - A "mirror" may exist at `github.com/<same-username>/<same-repo>`.
      - The mirror's default branch is kept in sync via Codeberg's platform sync — do not push it there directly.
      - The mirror may carry contributor branches so GitHub users can contribute there.
      
      Only if explicitly asked:
      - push to the mirror to keep it in sync
      - pull from mirror contributor branches to integrate contributions
      
      ===== docs/lang/Go.md =====
      ---
      title: Go Conventions
      weight: 60
      ---
      
      <!-- harnez:bundled -->
      # Go Conventions
      
      > **Who this is for** — anyone writing or reviewing Go in these repositories. Reference material: grep it, don't read it.
      >
      > **Read this if** — you are about to add a dependency, a global, or a test framework.
      >
      > **Takeaways**
      > 1. Standard library first; every dependency needs a reason and approval (`cobra` for CLI, `yaml.v3` for config).
      > 2. No package-level mutable state and no `panic()` outside boot.
      > 3. Table-driven tests in the same package, standard `testing` only.
      
      ---
      
      ## Language & Deps
      - **Modern Go**: Leverage current features like `any` and generics, but only where they explicitly reduce boilerplate/noise.
      - **Minimise external deps**: Default to the Go Standard Library. If you need an external library, get approval first.
      - **Allowed & Recommended Libraries**: `github.com/spf13/cobra` for CLI tools, `gopkg.in/yaml.v3` for YAML config/spec files, and `github.com/google/jsonschema-go/jsonschema` for JSON Schema validation and schema-aware tooling. Use the latter with `yaml.v3` when validating YAML documents against JSON Schemas.
      - **No Heavy Frameworks**: No ORMs (write raw SQL), no logging frameworks (use standard `log`), no Dependency Injection (DI) containers (pass dependencies explicitly).
      
      ## Project Layout
      - **Small**: Root `main.go` with features split by concern (`apply.go`, `config.go`, `status.go`).
      - **Med**: Root `main.go` with `internal/` sub-packages; no `pkg/`.
      - **Large/Multi**: Tools in `cmd/<name>/main.go` with features in split files or `internal/`.
      - **Embed static assets & specs**: Use `//go:embed`.
      - **Never commit binaries**: `go build` drops binaries in the repo root.
        Add it to `.gitignore` at project setup time:
        ```
        # ignore Go binaries
        /mybinary
        ```
        Use the `BINARY` variable from the Makefile as the canonical name so the `.gitignore` entry and the build output always match.
      
      ## Workspace Isolation
      
      - Go automatically applies the nearest enclosing `go.work` to every descendant directory. A
        persistent workspace for sibling-module development can therefore break an unrelated nested
        repository whose modules are absent from that workspace.
      - `harnez init` probes `go env GOWORK` and the active workspace's parsed module list. When an
        enclosing workspace omits any Go module found in the target repository, init creates a minimal
        project-local `go.work` that uses all of the repository's modules. Existing local workspaces are
        always preserved, and init creates nothing when there is no module or no demonstrated hazard.
      - A project-local workspace is the normal isolation boundary. To deliberately use a different
        cross-repository workspace for one command, select it explicitly, for example
        `GOWORK=/path/to/go.work go test ./...`. Release builds still use their separate
        `GOWORK=off` policy unless `harnez release --allow-workspace` is passed.
      - When a module has a local `replace` pointing at a sibling module, include both modules in the
        project-local workspace (for example, `use ( ./ ../loom )`). A workspace does not make a
        relative replacement path portable: copied projects still need the sibling directory, or a
        published module version. Keep the `replace` in `go.mod` when `GOWORK=off` builds still need the
        local fallback.
      - After copying a project, validate every relative replacement with `go list ./...`. A missing
        sibling replacement can make `gopls` repeatedly diagnose the module and make editor saves
        appear to hang while the workspace is loading.
      
      ## Spec-Driven Apps
      - Avoid hard-coding application configuration, UI labels, controls, text, icons, or layout variables in Go code.
      - Define them in YAML specs under `spec/` (e.g. `spec/strings.yaml`, `spec/layout.yaml`, `spec/controls.yaml`) with `$schema` in `spec/schemas/`.
      - Embed `spec/` into the Go binary (`//go:embed`); it **IS** part of the code!
      - Write compiler/unit test assertions to verify Go structs match specs.
      
      ## CLI & Releases
      - Use Cobra; one `*cobra.Command` per verb, flags defined on that command.
      - Use `RunE` instead of `Run` — return errors, don't `os.Exit` inside commands.
      - Set `SilenceUsage: true` on commands where error is not a usage mistake.
      - **Version Wiring**: Keep `var Version = "..."` in `version.go` (synced automatically by `harnez release` from `version.yaml`) and wire `rootCmd.Version = Version`.
      - **Man Pages**: Provide `<tool> man` (stdout roff) and `<tool> man --install` (writes `.1` files to `~/.local/share/man/man1` or `/usr/local/share/man/man1`) via `cobra/doc` so `go install` users get man pages. Only read `@docs/ManPages.md` when first implementing or troubleshooting man page setup.
      - **Releases**: Provide a thin `release: check ⚙️` recipe that delegates to `harnez release`. See `@docs/GoRelease.md`.
      
      ## State Management
      - **No package-level mutable variables**: Pass state explicitly via function arguments or state structs (e.g. `type App struct { client *http.Client }`). Mutable global state creates hidden coupling and breaks tests.
      - **Safe `init()` usage**: Use `init()` only for static, side-effect-free registrations (e.g. `flag.Var`). Never connect to databases, services, or load files during initialization.
      
      ## Error Handling
      - **Wrap errors with context**: Use `fmt.Errorf("module: %w", err)` to preserve the original error while giving readable context.
      - **No panic**: Never use `panic()` except for truly unrecoverable boot-time configurations.
      - **Return errors**: Let errors bubble up to `main()` where they are printed and exited.
      
      ## Types & Style
      - Unexported types for internal results; exported only when crossing package boundary.
      - Pointer fields (`*bool`, `*int`) for optional struct values; add `boolPtr`/`intPtr` helpers.
      - Section banners: `// ── Section name ────────────────────────────────────────────`
      
      ## Strings, Runes & Terminal Width
      - Never index or slice a string by byte offset (`s[i:j]`) for human-readable text or aligned output.
      - Never use `len(s)` as a visual column count: it measures bytes, not terminal cells.
      - Use `[]rune(s)` for character-count logic and `runewidth.StringWidth` for terminal width.
      - Strip ANSI escapes before measuring width; test rendered cell width, not rune count.
      - The existing `internal/usage` width assertions are the canonical pattern for TUI output.
      - Doc comments on all exported symbols.
      
      ## Output Discipline
      - Functions return values; callers own printing.
      - Indent output messages for visual hierarchy:
        - Print changed items with two spaces: `  wrote README.md`
        - Print sub-details with four spaces: `    size: 1.2 KB`
      
      ## Unit Tests
      - Use standard Go table-driven tests (`t.Run`).
      - Put tests in the same package (e.g. `package main`) to test unexported components easily.
      - Helpers: `t.Helper()`, `t.Fatalf` for setup failures, `t.Errorf` for assertion failures.
      - Standard library `testing` only — no external testing frameworks or mock generators.
      
      ===== docs/lang/Make.lite.md =====
      <!-- harnez:variant=lite -->
      # Make Rules (Lite)
      
      Language assumed: Go. Adapt to others.
      
      ## Structure
      
      - 1st target = default goal = `help` (bare `make` prints usage)
      - every target declared phony via the `⚙️`/`🤖` sentinel prereq — never per-target `.PHONY` names
      - 1 blank line between targets
      - every help-visible target carries `  # description` on its rule header line
      
      ## Variables
      
      ```makefile
      BINARY  := harnez              # output binary name
      CONFIG  := config.yaml         # default config file
      TARGET  := $(HOME)/.claude     # install target dir
      PROJECT := .                   # project root (tool's -p)
      PREFIX  ?= /usr/local          # ?= env-overridable; := immediate, use for all others
      
      export MYAPP_SOME_FEATURE=1
      # align the = signs
      ```
      
      ## Sentinels
      
      ```makefile
      .PHONY: ⚙️ 🤖
      ```
      
      - `build: ⚙️  # ...` / `help: 🤖  # ...` — sentinel as prereq on EVERY target makes Make treat
        all as phony without naming each twice
      - `🤖` = managed by harnez (reconciled/updated, e.g. `help`)
      - `⚙️` = manual/generated once (`build`, `test`, `release`); harnez never overwrites
      
      ## help
      
      ```makefile
      _prim := \033[36m
      _rst  := \033[0m
      
      help: 🤖  # show this help
      	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
      	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'
      ```
      
      ## build + action targets
      
      ```makefile
      build: ⚙️  # build the binary
      	go build -o $(BINARY) .
      
      apply: ⚙️ build  # apply config.yaml to the Claude Code config dir
      	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)
      ```
      
      - action targets depend on `build` -> binary always fresh; `build` rebuilds only on source change
      - always run `./$(BINARY)` (locally built), never `$PATH`'s; user may override this rule
      
      ## install (Go) — do local + try global
      
      ```makefile
      install: ⚙️ build  # install the binary to PREFIX/bin (default: /usr/local/bin)
      	go install .
      	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && \
      	  echo "✅ Installed for all users" || echo "⚠️ System install failed"
      ```
      
      `go install` -> `$(GOPATH)/bin` (user-local); `sudo install -m 0755` -> `$(PREFIX)/bin`
      (system-wide); `|| echo` degrades gracefully when sudo is unavailable.
      
      ## check
      
      ```makefile
      check: ⚙️  # run static analysis and tests
      	go vet ./...
      	go test ./...
      
      check-fast: ⚙️  # fast local feedback loop
      	go test ./...
      
      test: ⚙️ check  # alias for check
      ```
      
      - `check` = standard verification target of the dev-flow docs; `go vet` ALWAYS before `go test`
        (vet catches what tests miss)
      - keep `test` as compat alias where a repo already exposes it
      - add `check-fast` when full checks are slow: broad coverage, cheap settings (e.g. trafficsim
        runs one focused model via `MODEL=...`)
      
      ## deployment parity
      
      Any project with mutating provisioners (pushes binary/config/schedule to a remote host) MUST
      expose these four, so deploy/verify is never ad-hoc SSH one-liners. See the optional
      `deployment-transparency` practice for why `run`/`status` must probe the live host instead of
      inferring success from a completed `deploy`.
      
      ```makefile
      deploy: ⚙️ build  # deploy binary, configs, and cron schedules (DRY=1 for dry-run)
      	@scripts/deploy.sh $(if $(DRY),--dry-run)
      
      run: ⚙️  # query live deployment health (process, service, or job status)
      	@scripts/deploy.sh --status
      
      status: ⚙️ run  # alias for run
      
      backup: ⚙️  # sync state snapshots from the remote host
      	@scripts/backup.sh
      ```
      
      - `deploy [DRY=1]` — ships binary, configs, cron/systemd schedules; `DRY=1` = REAL dry-run
        against the remote host, not a no-op
      - `run`/`status` — read-only: query real remote process table / systemd units / crontab, never
        local repo state; keep `status` as alias where `run` already exists
      - `backup` — pull state snapshots (config overlays, data) down before a risky deploy
      - all four touch a real remote host — like `make smoke` (@docs/AgenticLoop.md): safe to define,
        run only when you intend the live effect
      
      ===== docs/lang/Make.md =====
      ---
      title: Make Conventions
      weight: 62
      ---
      
      <!-- harnez:bundled -->
      # Make Conventions
      
      > **Who this is for** — anyone adding a target to a project Makefile. Reference material: grep it, don't read it.
      >
      > **Read this if** — you need the `⚙️` sentinel or the self-documenting `help` recipe.
      >
      > **Takeaways**
      > 1. `help` is the first target; bare `make` prints usage.
      > 2. The `⚙️` sentinel (and `🤖` for managed targets) replaces per-target `.PHONY` bookkeeping.
      > 3. Express order as dependencies, and always run the locally built binary (`./$(BINARY)`).
      
      ---
      
      Default language assumed: Go.
      Apply to other languages accordingly.
      
      ## Structure
      
      - First target is the default goal — always `help`
      - All targets declared `.PHONY` using the `⚙️` sentinel trick (see below)
      - One blank line between targets
      
      ## Variables
      
      Example:
      
      ```makefile
      BINARY  := harnez              # output binary name
      CONFIG  := config.yaml         # default config file
      TARGET  := $(HOME)/.claude     # installation target dir
      PROJECT := .                   # project root (passed to tool as -p)
      PREFIX  ?= /usr/local          # overridable install prefix
      
      export MYAPP_SOME_FEATURE=1
      ```
      
      - Use `:=` for immediate assignment (most vars)
      - Use `?=` for env-overridable vars (`PREFIX`)
      - Align `=` signs for readability
      
      ## Phony declaration — `⚙️ 🤖` sentinels
      
      ```makefile
      .PHONY: ⚙️ 🤖  # ⚙️ = manual/once, 🤖 = managed
      ```
      
      Adding one of the sentinels as a prerequisite on every target (e.g. `build: ⚙️  # ...` or `help: 🤖  # ...`) causes Make to treat all targets as phony without listing each name twice.
      
      - `🤖` represents targets actively **managed** (reconciled/updated) by `harnez` (like `help`).
      - `⚙️` represents targets **manually** defined or generated once (like `build`, `test`, `release`), which `harnez` will not automatically overwrite.
      
      ## Self-documenting help target
      
      ```makefile
      _prim := \033[36m
      _rst  := \033[0m
      
      help: 🤖  # show this help
      	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
      	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'
      ```
      
      Every target that should appear in help gets a `  # description` comment on the same line as the rule header. `help` scrapes them automatically.
      
      ## Build dependency pattern
      
      Action targets depend on `build` so the binary is always fresh:
      
      ```makefile
      build: ⚙️  # build the binary
      	go build -o $(BINARY) .
      
      apply: ⚙️ build  # apply config.yaml to the Claude Code config directory
      	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)
      ```
      
      - `build` rebuilds only when sources change (Make's normal rules apply)
      - Action targets invoke `./$(BINARY)` — the locally-built binary, not the one on `$PATH`.
        If needed, the user can override this rule if development is close to his system.
      
      ## Install target (Go)
      
      ```makefile
      install: ⚙️ build  # install the binary to PREFIX/bin (default: /usr/local/bin)
      	go install .
      	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && \
      	  echo "✅ Installed for all users" || echo "⚠️ System install failed"
      ```
      
      Install approach is usually: do local + try global
      - `go install` puts the binary in `$(GOPATH)/bin` (user-local)
      - `sudo install -m 0755` copies to `$(PREFIX)/bin` for system-wide availability
      - `|| echo …` degrades gracefully when `sudo` is unavailable
      
      ## Check target
      
      ```makefile
      check: ⚙️  # run static analysis and tests
      	go vet ./...
      	go test ./...
      
      check-fast: ⚙️  # fast local feedback loop
      	go test ./...
      
      test: ⚙️ check  # alias for check
      ```
      
      `make check` is the standard verification target used by development-flow docs.
      Always run `go vet` before `go test`; vet catches issues tests may not exercise.
      Keep `make test` as a compatibility alias when a repo already exposes it.
      Add `make check-fast` when full checks are slow; it should keep broad coverage but use cheap settings. Example: `trafficsim` runs one focused model via `MODEL=...`.
      
      ## Deployment target parity
      
      Any project with mutating provisioners (pushes a binary, config, or schedule to a remote host)
      must expose the same four self-documenting targets, so deploy/verify is never ad hoc SSH
      one-liners. For projects with remote deployment, install the optional
      `deployment-transparency` practice
      for why the query targets (`run`/`status`) must probe the live host rather than assume success
      from a completed `deploy`.
      
      ```makefile
      deploy: ⚙️ build  # deploy binary, configs, and cron schedules (DRY=1 for dry-run)
      	@scripts/deploy.sh $(if $(DRY),--dry-run)
      
      run: ⚙️  # query live deployment health (process, service, or job status)
      	@scripts/deploy.sh --status
      
      status: ⚙️ run  # alias for run
      
      backup: ⚙️  # sync state snapshots from the remote host
      	@scripts/backup.sh
      ```
      
      - `make deploy [DRY=1]` — ships binary, configs, and cron/systemd schedules; `DRY=1` must perform
        a real dry-run against the remote host, not a no-op.
      - `make run` / `make status` — read-only: query the actual remote process table, systemd units, or
        crontab, not the local repo state. Keep `status` as an alias when a repo already exposes `run`.
      - `make backup` — sync state snapshots (config overlays, data) down from the remote host before a
        risky deploy.
      - Every target here queries or mutates a real remote host — treat it like `make smoke` (see
        `@docs/AgenticLoop.md`): safe to define, but only run when you intend the live effect.
      
      ===== docs/lang/ManPages.md =====
      ---
      title: Man Pages for Go CLIs
      weight: 65
      ---
      
      <!-- harnez:bundled -->
      # Man Pages for Go CLIs
      
      > **Who this is for** — developers creating or maintaining CLI tools in Go who want first-class manual page support across `go install`, pre-built binaries, release tarballs, and system packages.
      >
      > **Takeaways**
      > 1. `go install` only installs the binary into `$GOBIN` — giving the binary a self-installing `<cmd> man --install` subcommand bridges the gap.
      > 2. Generate roff man pages directly from Cobra commands (`cobra/doc`) to eliminate doc drift.
      > 3. Target XDG `~/.local/share/man/man1` for unprivileged users and `/usr/local/share/man/man1` for root.
      
      ---
      
      ## 1. Problem: The `go install` Delivery Gap
      
      `go install` compiles and places only the executable binary into `$GOBIN` (or `~/go/bin`). It provides no mechanism to install auxiliary assets like roff man pages, shell completions, or systemd units.
      
      As a result:
      - Standalone roff files (e.g. `cmd.1`) checked into the repo frequently drift out of sync with code changes and flags.
      - Users installing via `go install` or downloading standalone binaries get working commands but missing `man <cmd>` pages.
      
      ---
      
      ## 2. Architecture: The 3-Tiered Man Page Pattern
      
      ```
      ┌────────────────────────────────────────────────────────┐
      │ Cobra Command Tree (Single Source of Truth)            │
      │ github.com/spf13/cobra/doc.GenMan / GenManTree         │
      └───────────┬──────────────────┬─────────────────┬───────┘
                  │                  │                 │
                  ▼                  ▼                 ▼
         ┌─────────────────┐ ┌───────────────┐ ┌───────────────┐
         │ 1. CLI Command  │ │ 2. Makefile   │ │ 3. Packaging  │
         │  `<tool> man`   │ │`make install` │ │ GoReleaser /  │
         │ `<tool> man -i` │ │  `make man`   │ │  NFPM / DEB   │
         └─────────────────┘ └───────────────┘ └───────────────┘
      ```
      
      By generating the roff representation dynamically from Cobra's command hierarchy, flag definitions and documentation remain strictly in sync with the codebase.
      
      ---
      
      ## 3. Subcommand Design: `<cmd> man`
      
      Every CLI tool should provide a `man` subcommand with two primary workflows:
      
      ### A. Stdout Pipe (Zero-Install Inspection)
      Prints the generated roff manual to standard output:
      ```bash
      # View without installing
      psync man | man -l -
      
      # Redirect to arbitrary destination
      psync man > /tmp/psync.1
      ```
      
      ### B. Self-Installation (`--install`)
      Generates the entire man tree (including subcommands) and writes `.1` files directly into the platform's standard manpath:
      ```bash
      psync man --install
      man psync
      ```
      
      ### Standard Path Resolution:
      - **Unprivileged User**: `$XDG_DATA_HOME/man/man1` (defaults to `~/.local/share/man/man1`). Modern Linux distributions include `~/.local/share/man` in default `man` search paths automatically.
      - **Root / Sudo** (`os.Geteuid() == 0`): `/usr/local/share/man/man1`.
      - **Custom Override**: `--dir <path>` for custom packaging setups.
      
      ---
      
      ## 4. Go Implementation Blueprint
      
      Create a dedicated `man.go` file in the root CLI package:
      
      ```go
      package main
      
      import (
      	"fmt"
      	"io"
      	"os"
      	"path/filepath"
      
      	"github.com/spf13/cobra"
      	"github.com/spf13/cobra/doc"
      )
      
      func newManCmd(rootCmd *cobra.Command) *cobra.Command {
      	var (
      		install bool
      		dir     string
      	)
      	cmd := &cobra.Command{
      		Use:   "man",
      		Short: "generate and print or install roff man pages",
      		Long: `man generates standard roff/troff man pages.
      
      Without flags, it prints the roff man page to stdout (e.g. tool man | man -l -).
      With --install, it writes .1 man pages to ~/.local/share/man/man1 (or /usr/local/share/man/man1 if root).`,
      		Args: cobra.NoArgs,
      		RunE: func(cmd *cobra.Command, args []string) error {
      			if install || dir != "" {
      				return installManPages(rootCmd, dir, cmd.OutOrStdout())
      			}
      			header := &doc.GenManHeader{
      				Title:   "TOOLNAME",
      				Section: "1",
      				Source:  "toolname " + Version,
      				Manual:  "General Commands Manual",
      			}
      			return doc.GenMan(rootCmd, header, cmd.OutOrStdout())
      		},
      	}
      	// Note: Avoid shorthand '-i' if the root command has a persistent '-i' (e.g. --interval)
      	cmd.Flags().BoolVar(&install, "install", false, "install man pages into standard system or user man directory")
      	cmd.Flags().StringVarP(&dir, "dir", "d", "", "custom target directory for man pages (implies --install)")
      	return cmd
      }
      
      func defaultManDir() string {
      	if os.Geteuid() == 0 {
      		return "/usr/local/share/man/man1"
      	}
      	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
      		return filepath.Join(xdg, "man", "man1")
      	}
      	if home, err := os.UserHomeDir(); err == nil && home != "" {
      		return filepath.Join(home, ".local", "share", "man", "man1")
      	}
      	return "/usr/local/share/man/man1"
      }
      
      func installManPages(rootCmd *cobra.Command, targetDir string, out io.Writer) error {
      	if targetDir == "" {
      		targetDir = defaultManDir()
      	}
      	if err := os.MkdirAll(targetDir, 0755); err != nil {
      		return fmt.Errorf("creating man directory %s: %w", targetDir, err)
      	}
      	header := &doc.GenManHeader{
      		Title:   "TOOLNAME",
      		Section: "1",
      		Source:  "toolname " + Version,
      		Manual:  "General Commands Manual",
      	}
      	if err := doc.GenManTree(rootCmd, header, targetDir); err != nil {
      		return fmt.Errorf("generating man pages in %s: %w", targetDir, err)
      	}
      	fmt.Fprintf(out, "Installed man pages to %s\nRun 'man toolname' to view.\n", targetDir)
      	return nil
      }
      ```
      
      ---
      
      ## 5. Gotchas & Lessons Learned
      
      ### Flag Inheritance Collisions
      Cobra propagates `PersistentFlags()` down to all child subcommands. If the root command registers a persistent flag with shorthand `-i` (e.g. `-i, --interval`), registering `-i, --install` on `newManCmd` causes a runtime panic on initialization:
      ```
      panic: unable to redefine 'i' shorthand in "man" flagset: it's already used for "interval" flag
      ```
      **Rule**: Use long-form `--install` without shorthand `-i` on subcommands whenever the root command reserves persistent flags.
      
      ### Dependencies in `go.mod`
      Importing `github.com/spf13/cobra/doc` introduces transitive dependencies (`github.com/cpuguy83/go-md2man/v2` and `gopkg.in/yaml.v3`). Run `go mod tidy` after wiring `cobra/doc`.
      
      ---
      
      ## 6. Build & Packaging Integration
      
      ### Makefile
      Wire man page generation into standard development workflows:
      
      ```makefile
      man: ⚙️ build  # generate roff man page
      	./$(BINARY) man > $(BINARY).1
      
      install: ⚙️ build  # install binary to GOBIN and install man page
      	go install .
      	./$(BINARY) man --install
      
      clean: ⚙️
      	rm -f $(BINARY) $(BINARY).1
      ```
      
      ### `.gitignore`
      Ignore generated roff files in the repo root:
      ```
      /*.1
      ```
      
      ### GoReleaser & NFPM Packages
      In `.goreleaser.yaml`, ensure man pages are built before packaging or included in distribution artifacts:
      ```yaml
      nfpms:
        - id: packages
          package_name: toolname
          contents:
            - src: dist/man/toolname.1
              dst: /usr/share/man/man1/toolname.1
      ```
      
      ===== docs/lang/Markdown.md =====
      ---
      title: Markdown Conventions
      weight: 64
      ---
      
      <!-- harnez:bundled -->
      # Markdown Conventions
      
      ## File naming
      
      **Evergreen docs** — content that doesn't expire (references, conventions, guides):
      
      ```
      docs/Permissions.md
      docs/Go.md
      docs/Worktrees.md
      ```
      
      Use **PascalCase**. No dates, no issue numbers.
      
      **Ephemeral docs** — issues, reports, reviews, ADRs, changelogs:
      
      ```
      issues/001-summary-of-issue.md
      reviews/2026-06-auth-refactor.md
      ```
      
      Use **kebab-case** with an optional numeric or date prefix for ordering.
      
      ## Content
      
      - One `#` title per file, matching the filename concept
      - Prefer bullet lists over tables for sparse data
      - Keep files token-efficient: no redundant prose, no section headers that restate the bullet below them
      
      ## Terminal hyperlinks (OSC 8): label text by context
      
      Always prefer an OSC 8 hyperlink over a raw path. The *visible label* rule depends on context:
      
      - **In tables**: use a short/truncated label as the visible text; put the full path in the OSC 8
        target. A long filename as the label blows up column width and breaks alignment.
      - **In flowing prose**: show the **full path** as the visible text, with the OSC 8 hyperlink
        layered on for click-to-open. The user needs to visually scan and drag-select/copy the path
        directly — a shortened label defeats that. Truncate only if the full path is genuinely too long
        for inline prose.
      
      ## Diagrams: chat vs. docs
      
      ### In chat / pairing conversations:
      - Do not emit ` ```mermaid ` blocks unless asked; terminals and chat UIs may render them as raw markup
      - For simple flows, use `1. 2. 3. if X goto 2.; 4. ...` and skip the diagram completely
      - Use compact ASCII box-and-arrow diagrams inside a ` ```text ` fence instead:
        - **Dynamic terminal sizing**: Adapt diagram width to the terminal/pane width $W$.
        - **5% Right margin buffer**: Always leave at least a **5% buffer** on the right side ($\text{width} \le 0.95 \times W$) so terminals never auto-wrap and break rectangular borders.
        - **Max width ceiling (120 cols)**: Cap diagram width at **120 columns** max, even on ultra-wide terminals.
        - **Vertical stacking**: If a flow exceeds the safe width, stack boxes vertically rather than spreading wide horizontally.
      
        ```text
        ┌─────────────┐       ┌─────────────┐
        │     Foo     │ ────> │     Bar     │
        └─────────────┘       └─────────────┘
        ```
      
      ### In MD Files/Evergreen `docs/`
       ` ```mermaid ` diagrams remain allowed and encouraged for formal architecture/sequence flows.
      
      ===== docs/lang/Rust.md =====
      ---
      title: Rust Conventions
      weight: 65
      ---
      
      <!-- harnez:bundled -->
      # Rust Conventions
      
      ## Tooling
      - Always use standard formatter (`cargo fmt`) and linter (`cargo clippy`).
      - Target the current stable compiler version.
      
      ## Dependencies
      - Keep external dependencies minimal.
      - Use `clap` (with derive feature) for CLI arguments.
      - Use `serde` for serialization/deserialisation.
      - Use `anyhow` for application-level error handling, and `thiserror` for library boundaries.
      - Avoid heavy runtime frameworks (like `tokio`) unless asynchronous operations are explicitly required.
      
      ## Error Handling
      - Avoid `panic!`, `unwrap()`, or `expect()` in library and production code.
      - If an operation can fail, return a `Result` or `Option`.
      - In test code, `unwrap()` is permitted.
      
      ## Project Structure
      - For applications, put core logic in a library (`src/lib.rs`) and CLI handling in the binary (`src/main.rs`).
      - Unit tests go in the same file as the code under a `tests` module:
        ```rust
        #[cfg(test)]
        mod tests {
            use super::*;
            // ...
        }
        ```
      - Integration tests go in the `tests/` directory.
      
      ## Unsafe Code
      - Avoid `unsafe` block unless strictly required for Wayland protocol integrations or C bindings.
      - All `unsafe` blocks must be documented with a `// SAFETY:` comment explaining why the invariants are met.
      
      ===== docs/lang/Zig.md =====
      ---
      title: Zig Conventions
      weight: 63
      ---
      
      <!-- harnez:bundled -->
      # Zig Conventions
      
      > ⚠️ **Version banner: Zig 0.16.0, verified 2026-07. These APIs move.**
      > The conventions in the first section are stable house rules. The API shapes in
      > the second section (`pipe2`, `StdIo.file` flags, `Permissions.fromMode`,
      > peer-type resolution) are notes from a moving target — they were true against
      > 0.16.0 and the compiler, not this page, is the authority. If your compiler
      > disagrees, your compiler is right.
      
      > **Who this is for** — anyone writing Zig in these repositories. Reference material: grep it, don't read it.
      >
      > **Read this if** — you hit a 0.16.0 API that does not look like the docs.
      >
      > **Takeaways**
      > 1. Allocations are always explicit; hot paths allocate nothing.
      > 2. Utility binaries ship `-O ReleaseSmall`, under 1 MB, under 5 MB RSS.
      > 3. Treat every pinned API shape below as dated evidence, not as doctrine.
      
      ---
      
      ## Language & Allocation Conventions
      - **Explicit Allocation**: Never hide allocations. Pass an `Allocator` explicitly to functions that need to allocate memory.
      - **Zero-Allocation Cores**: For hot paths (such as fuzzy search engines or rendering loops), design a zero-allocation core. Prefetch data or use stack-allocated buffers.
      - **ReleaseSmall Optimization**: For utility binaries, optimize using `-O ReleaseSmall`. A typical statically linked binary should be under 1 MB and use less than 5 MB of RSS RAM.
      - **Clean Namespace Separation**: Avoid monolithic root files. Move helper logic to dedicated files and use `std.testing.refAllDecls(@This())` in your main test blocks to run tests in separated modules.
      
      ---
      
      ## Zig 0.16.0 API Pitfalls & Patterns
      
      ### 1. Pipes — Use `std.os.linux.pipe2` on Linux
      In Zig 0.16.0, `std.posix` does not expose `pipe2`. Use the Linux-specific syscall wrapper for non-blocking execution:
      ```zig
      var fds: [2]std.posix.fd_t = undefined;
      const flags = std.os.linux.O{ .NONBLOCK = true, .CLOEXEC = true };
      const rc = std.os.linux.pipe2(&fds, flags);
      switch (std.posix.errno(rc)) {
          .SUCCESS => {},
          else => return error.SystemResources,
      }
      ```
      
      ### 2. Spawning Subprocesses with Stdout Redirection
      When redirecting standard I/O to a file descriptor using `StdIo.file`, Zig 0.16.0 requires providing a `flags` field:
      ```zig
      var child = try std.process.spawn(io, .{
          .argv  = &argv,
          .stdin = .ignore,
          .stdout = .{ .file = .{
              .handle = pipe_fds[1],
              .flags  = .{ .nonblocking = false },
          }},
          .stderr = .ignore,
      });
      ```
      *Note: Close the write end of the pipe in the parent process immediately after spawning, or the read end will block forever on EOF.*
      
      ### 3. `close` and `read` — Use `std.posix.system.*`
      `std.posix.close` and `std.posix.read` do not exist directly in Zig 0.16. Use `system` sub-namespace for raw syscall wrappers:
      ```zig
      _ = std.posix.system.close(fd);
      ```
      
      ### 4. File Permissions via Permissions Enum
      Do not use raw numeric modes (e.g. `0o600`) when creating or modifying files in Zig 0.16.0. Use the `std.fs` types wrapper:
      ```zig
      const file = try std.fs.createFileAbsolute(io, path, .{
          .permissions = std.fs.File.Permissions.fromMode(0o600),
      });
      ```
      
      ### 5. Slice vs. Array Pointer Peer-Type Resolution
      Constant string literals (e.g., `""`) are of type `*const [0:0]u8`. Standard library formatting tools (like `std.fmt.bufPrint`) return mutable slices (`[]u8`). Peer-type resolution will fail if both are evaluated in a simple `catch` expression:
      ```zig
      // ❌ Compile error: incompatible types
      const s = std.fmt.bufPrint(&buf, "{s}", .{val}) catch "";
      
      // ✓ Fix: Use an if-expression to yield identical types
      const s = if (std.fmt.bufPrint(&buf, "{s}", .{val})) |res| res else |_| buf[0..0];
      ```
      
      ### 6. `std.mem.trim` Const/Mutable Safety
      `std.mem.trim` always returns `[]const u8`. If you are trimming a mutable buffer and require a mutable slice as a result, wrap the returned slice in `@constCast`:
      ```zig
      const trimmed = std.mem.trim(u8, buf[0..len], " \t\n\r");
      return @constCast(trimmed); // Safe: the underlying buffer is mutable
      ```
      
      ### 7. `gsettings` Floating-Point Output Precision
      `gsettings get org.gnome.desktop.interface text-scaling-factor` returns full IEEE 754 noise (`1.0999999999999999` instead of `1.1`). Parse first two decimal digits and round to nearest tenth:
      ```zig
      const d1: usize = if (d + 1 < s.len) (s[d + 1] - '0') else 0;
      const d2: usize = if (d + 2 < s.len) (s[d + 2] - '0') else 0;
      const frac = if (d2 >= 5) d1 + 1 else d1;
      const scale10 = int_part * 10 + frac; // e.g. 11 for 1.1
      ```
      
      ### 8. Calling libc `unsetenv()` desyncs `std.process.spawn`'s own `environ` cache
      `std.Io.Threaded` (default `Io`) caches its view of `environ`. Calling libc's `unsetenv()` directly mutates libc's table while Zig's cache goes stale, causing subsequent `std.process.spawn` calls to segfault.
      - Avoid calling `unsetenv()` after `std.process.spawn`. Perform all initial spawns first, then treat `unsetenv()` as one-way.
      - Call a warm-up `std.debug.print("", .{})` prior to `unsetenv()` to avoid a lazy debug-info scan deadlock.
      
      ### 9. dlopen'd C Libraries with Struct Fields
      - When writing `extern struct` bindings for dynamic libraries (e.g. font shaping), if the API exposes public struct fields (e.g. `.cols`, `.advance.x`), transcribe the exact layout from the real C header rather than treating handles as `?*anyopaque`.
      - For opaque-pointer C APIs, `?*anyopaque` is safe and eliminates header dependencies.

## Activity

    $ /usr/bin/zsh -lc "⚙ bash -c 'for f in docs/lang/Git.md docs/lang/Go.md docs/lang/Make.lite.md docs/lang/Make.md; do printf '\\''\\n===== %s =====\\n'\\'' \""'$f"; sed -n '"'\\''1,260p'\\'' \""'$f"; done'"'"
    status: Completed · exit 0
      
      ===== docs/lang/Git.md =====
      ---
      title: Git Conventions
      weight: 63
      ---
      
      <!-- harnez:bundled -->
      # Git conventions
      
      Applies unless overridden by project-local instructions for coding agents,
      commonly stored in an `AGENTS.md` file at the repository root.
      
      ## Workflow
      - Work local, work on the repo's default branch (usually `main`, sometimes `master`) —
        no local feature branches unless the project's Repo Setup says otherwise.
      - Commit proactively:
        - Default preference: commit autonomously after intermediate steps once tests are clean, after finished features (with review pass), between iteration attempts on a stuck bug, and immediately after filing issue-tracker entries.
        - Ask-first harness interaction: If running in a harness that mandates explicit user confirmation for `git commit`, ask the user **upfront before starting work / during session kickoff** for authorization to commit local checkpoints proactively. This enables the user to step away without returning to uncommitted progress.
        - Orchestrator delegation: When an orchestrating agent operates under an ask-first rule, it must clarify commit authority with the user at kickoff and specify whether spawned subagents commit directly or return diffs for the orchestrator to commit on their behalf.
      - Review before commit:
        - small fixes: commit directly if all tests pass and existing test assertions remain intact
        - non-trivial changes / features: perform a dedicated review pass (host orchestration or a fresh reviewer subagent) before committing to verify test rigor, doc/ticket accuracy, and codebase clarity
      - Do not push unless asked.
      - Do not create remote branches or PRs unless asked.
      
      ## Commit messages
      - Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary):
        `type(scope)?: summary`, e.g. `feat(cli): add init command`.
      - Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
      
      ## Hygiene
      - Do not commit secrets.
      - Do not commit large files unless tracked via git-lfs.
      - Do not commit PII.
      
      ## Remotes
      - Canonical origin: `codeberg.org/<username>/<reponame>`.
      - A "mirror" may exist at `github.com/<same-username>/<same-repo>`.
      - The mirror's default branch is kept in sync via Codeberg's platform sync — do not push it there directly.
      - The mirror may carry contributor branches so GitHub users can contribute there.
      
      Only if explicitly asked:
      - push to the mirror to keep it in sync
      - pull from mirror contributor branches to integrate contributions
      
      ===== docs/lang/Go.md =====
      ---
      title: Go Conventions
      weight: 60
      ---
      
      <!-- harnez:bundled -->
      # Go Conventions
      
      > **Who this is for** — anyone writing or reviewing Go in these repositories. Reference material: grep it, don't read it.
      >
      > **Read this if** — you are about to add a dependency, a global, or a test framework.
      >
      > **Takeaways**
      > 1. Standard library first; every dependency needs a reason and approval (`cobra` for CLI, `yaml.v3` for config).
      > 2. No package-level mutable state and no `panic()` outside boot.
      > 3. Table-driven tests in the same package, standard `testing` only.
      
      ---
      
      ## Language & Deps
      - **Modern Go**: Leverage current features like `any` and generics, but only where they explicitly reduce boilerplate/noise.
      - **Minimise external deps**: Default to the Go Standard Library. If you need an external library, get approval first.
      - **Allowed & Recommended Libraries**: `github.com/spf13/cobra` for CLI tools, `gopkg.in/yaml.v3` for YAML config/spec files, and `github.com/google/jsonschema-go/jsonschema` for JSON Schema validation and schema-aware tooling. Use the latter with `yaml.v3` when validating YAML documents against JSON Schemas.
      - **No Heavy Frameworks**: No ORMs (write raw SQL), no logging frameworks (use standard `log`), no Dependency Injection (DI) containers (pass dependencies explicitly).
      
      ## Project Layout
      - **Small**: Root `main.go` with features split by concern (`apply.go`, `config.go`, `status.go`).
      - **Med**: Root `main.go` with `internal/` sub-packages; no `pkg/`.
      - **Large/Multi**: Tools in `cmd/<name>/main.go` with features in split files or `internal/`.
      - **Embed static assets & specs**: Use `//go:embed`.
      - **Never commit binaries**: `go build` drops binaries in the repo root.
        Add it to `.gitignore` at project setup time:
        ```
        # ignore Go binaries
        /mybinary
        ```
        Use the `BINARY` variable from the Makefile as the canonical name so the `.gitignore` entry and the build output always match.
      
      ## Workspace Isolation
      
      - Go automatically applies the nearest enclosing `go.work` to every descendant directory. A
        persistent workspace for sibling-module development can therefore break an unrelated nested
        repository whose modules are absent from that workspace.
      - `harnez init` probes `go env GOWORK` and the active workspace's parsed module list. When an
        enclosing workspace omits any Go module found in the target repository, init creates a minimal
        project-local `go.work` that uses all of the repository's modules. Existing local workspaces are
        always preserved, and init creates nothing when there is no module or no demonstrated hazard.
      - A project-local workspace is the normal isolation boundary. To deliberately use a different
        cross-repository workspace for one command, select it explicitly, for example
        `GOWORK=/path/to/go.work go test ./...`. Release builds still use their separate
        `GOWORK=off` policy unless `harnez release --allow-workspace` is passed.
      - When a module has a local `replace` pointing at a sibling module, include both modules in the
        project-local workspace (for example, `use ( ./ ../loom )`). A workspace does not make a
        relative replacement path portable: copied projects still need the sibling directory, or a
        published module version. Keep the `replace` in `go.mod` when `GOWORK=off` builds still need the
        local fallback.
      - After copying a project, validate every relative replacement with `go list ./...`. A missing
        sibling replacement can make `gopls` repeatedly diagnose the module and make editor saves
        appear to hang while the workspace is loading.
      
      ## Spec-Driven Apps
      - Avoid hard-coding application configuration, UI labels, controls, text, icons, or layout variables in Go code.
      - Define them in YAML specs under `spec/` (e.g. `spec/strings.yaml`, `spec/layout.yaml`, `spec/controls.yaml`) with `$schema` in `spec/schemas/`.
      - Embed `spec/` into the Go binary (`//go:embed`); it **IS** part of the code!
      - Write compiler/unit test assertions to verify Go structs match specs.
      
      ## CLI & Releases
      - Use Cobra; one `*cobra.Command` per verb, flags defined on that command.
      - Use `RunE` instead of `Run` — return errors, don't `os.Exit` inside commands.
      - Set `SilenceUsage: true` on commands where error is not a usage mistake.
      - **Version Wiring**: Keep `var Version = "..."` in `version.go` (synced automatically by `harnez release` from `version.yaml`) and wire `rootCmd.Version = Version`.
      - **Man Pages**: Provide `<tool> man` (stdout roff) and `<tool> man --install` (writes `.1` files to `~/.local/share/man/man1` or `/usr/local/share/man/man1`) via `cobra/doc` so `go install` users get man pages. Only read `@docs/ManPages.md` when first implementing or troubleshooting man page setup.
      - **Releases**: Provide a thin `release: check ⚙️` recipe that delegates to `harnez release`. See `@docs/GoRelease.md`.
      
      ## State Management
      - **No package-level mutable variables**: Pass state explicitly via function arguments or state structs (e.g. `type App struct { client *http.Client }`). Mutable global state creates hidden coupling and breaks tests.
      - **Safe `init()` usage**: Use `init()` only for static, side-effect-free registrations (e.g. `flag.Var`). Never connect to databases, services, or load files during initialization.
      
      ## Error Handling
      - **Wrap errors with context**: Use `fmt.Errorf("module: %w", err)` to preserve the original error while giving readable context.
      - **No panic**: Never use `panic()` except for truly unrecoverable boot-time configurations.
      - **Return errors**: Let errors bubble up to `main()` where they are printed and exited.
      
      ## Types & Style
      - Unexported types for internal results; exported only when crossing package boundary.
      - Pointer fields (`*bool`, `*int`) for optional struct values; add `boolPtr`/`intPtr` helpers.
      - Section banners: `// ── Section name ────────────────────────────────────────────`
      
      ## Strings, Runes & Terminal Width
      - Never index or slice a string by byte offset (`s[i:j]`) for human-readable text or aligned output.
      - Never use `len(s)` as a visual column count: it measures bytes, not terminal cells.
      - Use `[]rune(s)` for character-count logic and `runewidth.StringWidth` for terminal width.
      - Strip ANSI escapes before measuring width; test rendered cell width, not rune count.
      - The existing `internal/usage` width assertions are the canonical pattern for TUI output.
      - Doc comments on all exported symbols.
      
      ## Output Discipline
      - Functions return values; callers own printing.
      - Indent output messages for visual hierarchy:
        - Print changed items with two spaces: `  wrote README.md`
        - Print sub-details with four spaces: `    size: 1.2 KB`
      
      ## Unit Tests
      - Use standard Go table-driven tests (`t.Run`).
      - Put tests in the same package (e.g. `package main`) to test unexported components easily.
      - Helpers: `t.Helper()`, `t.Fatalf` for setup failures, `t.Errorf` for assertion failures.
      - Standard library `testing` only — no external testing frameworks or mock generators.
      
      ===== docs/lang/Make.lite.md =====
      <!-- harnez:variant=lite -->
      # Make Rules (Lite)
      
      Language assumed: Go. Adapt to others.
      
      ## Structure
      
      - 1st target = default goal = `help` (bare `make` prints usage)
      - every target declared phony via the `⚙️`/`🤖` sentinel prereq — never per-target `.PHONY` names
      - 1 blank line between targets
      - every help-visible target carries `  # description` on its rule header line
      
      ## Variables
      
      ```makefile
      BINARY  := harnez              # output binary name
      CONFIG  := config.yaml         # default config file
      TARGET  := $(HOME)/.claude     # install target dir
      PROJECT := .                   # project root (tool's -p)
      PREFIX  ?= /usr/local          # ?= env-overridable; := immediate, use for all others
      
      export MYAPP_SOME_FEATURE=1
      # align the = signs
      ```
      
      ## Sentinels
      
      ```makefile
      .PHONY: ⚙️ 🤖
      ```
      
      - `build: ⚙️  # ...` / `help: 🤖  # ...` — sentinel as prereq on EVERY target makes Make treat
        all as phony without naming each twice
      - `🤖` = managed by harnez (reconciled/updated, e.g. `help`)
      - `⚙️` = manual/generated once (`build`, `test`, `release`); harnez never overwrites
      
      ## help
      
      ```makefile
      _prim := \033[36m
      _rst  := \033[0m
      
      help: 🤖  # show this help
      	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
      	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'
      ```
      
      ## build + action targets
      
      ```makefile
      build: ⚙️  # build the binary
      	go build -o $(BINARY) .
      
      apply: ⚙️ build  # apply config.yaml to the Claude Code config dir
      	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)
      ```
      
      - action targets depend on `build` -> binary always fresh; `build` rebuilds only on source change
      - always run `./$(BINARY)` (locally built), never `$PATH`'s; user may override this rule
      
      ## install (Go) — do local + try global
      
      ```makefile
      install: ⚙️ build  # install the binary to PREFIX/bin (default: /usr/local/bin)
      	go install .
      	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && \
      	  echo "✅ Installed for all users" || echo "⚠️ System install failed"
      ```
      
      `go install` -> `$(GOPATH)/bin` (user-local); `sudo install -m 0755` -> `$(PREFIX)/bin`
      (system-wide); `|| echo` degrades gracefully when sudo is unavailable.
      
      ## check
      
      ```makefile
      check: ⚙️  # run static analysis and tests
      	go vet ./...
      	go test ./...
      
      check-fast: ⚙️  # fast local feedback loop
      	go test ./...
      
      test: ⚙️ check  # alias for check
      ```
      
      - `check` = standard verification target of the dev-flow docs; `go vet` ALWAYS before `go test`
        (vet catches what tests miss)
      - keep `test` as compat alias where a repo already exposes it
      - add `check-fast` when full checks are slow: broad coverage, cheap settings (e.g. trafficsim
        runs one focused model via `MODEL=...`)
      
      ## deployment parity
      
      Any project with mutating provisioners (pushes binary/config/schedule to a remote host) MUST
      expose these four, so deploy/verify is never ad-hoc SSH one-liners. See the optional
      `deployment-transparency` practice for why `run`/`status` must probe the live host instead of
      inferring success from a completed `deploy`.
      
      ```makefile
      deploy: ⚙️ build  # deploy binary, configs, and cron schedules (DRY=1 for dry-run)
      	@scripts/deploy.sh $(if $(DRY),--dry-run)
      
      run: ⚙️  # query live deployment health (process, service, or job status)
      	@scripts/deploy.sh --status
      
      status: ⚙️ run  # alias for run
      
      backup: ⚙️  # sync state snapshots from the remote host
      	@scripts/backup.sh
      ```
      
      - `deploy [DRY=1]` — ships binary, configs, cron/systemd schedules; `DRY=1` = REAL dry-run
        against the remote host, not a no-op
      - `run`/`status` — read-only: query real remote process table / systemd units / crontab, never
        local repo state; keep `status` as alias where `run` already exists
      - `backup` — pull state snapshots (config overlays, data) down before a risky deploy
      - all four touch a real remote host — like `make smoke` (@docs/AgenticLoop.md): safe to define,
        run only when you intend the live effect
      
      ===== docs/lang/Make.md =====
      ---
      title: Make Conventions
      weight: 62
      ---
      
      <!-- harnez:bundled -->
      # Make Conventions
      
      > **Who this is for** — anyone adding a target to a project Makefile. Reference material: grep it, don't read it.
      >
      > **Read this if** — you need the `⚙️` sentinel or the self-documenting `help` recipe.
      >
      > **Takeaways**
      > 1. `help` is the first target; bare `make` prints usage.
      > 2. The `⚙️` sentinel (and `🤖` for managed targets) replaces per-target `.PHONY` bookkeeping.
      > 3. Express order as dependencies, and always run the locally built binary (`./$(BINARY)`).
      
      ---
      
      Default language assumed: Go.
      Apply to other languages accordingly.
      
      ## Structure
      
      - First target is the default goal — always `help`
      - All targets declared `.PHONY` using the `⚙️` sentinel trick (see below)
      - One blank line between targets
      
      ## Variables
      
      Example:
      
      ```makefile
      BINARY  := harnez              # output binary name
      CONFIG  := config.yaml         # default config file
      TARGET  := $(HOME)/.claude     # installation target dir
      PROJECT := .                   # project root (passed to tool as -p)
      PREFIX  ?= /usr/local          # overridable install prefix
      
      export MYAPP_SOME_FEATURE=1
      ```
      
      - Use `:=` for immediate assignment (most vars)
      - Use `?=` for env-overridable vars (`PREFIX`)
      - Align `=` signs for readability
      
      ## Phony declaration — `⚙️ 🤖` sentinels
      
      ```makefile
      .PHONY: ⚙️ 🤖  # ⚙️ = manual/once, 🤖 = managed
      ```
      
      Adding one of the sentinels as a prerequisite on every target (e.g. `build: ⚙️  # ...` or `help: 🤖  # ...`) causes Make to treat all targets as phony without listing each name twice.
      
      - `🤖` represents targets actively **managed** (reconciled/updated) by `harnez` (like `help`).
      - `⚙️` represents targets **manually** defined or generated once (like `build`, `test`, `release`), which `harnez` will not automatically overwrite.
      
      ## Self-documenting help target
      
      ```makefile
      _prim := \033[36m
      _rst  := \033[0m
      
      help: 🤖  # show this help
      	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
      	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'
      ```
      
      Every target that should appear in help gets a `  # description` comment on the same line as the rule header. `help` scrapes them automatically.
      
      ## Build dependency pattern
      
      Action targets depend on `build` so the binary is always fresh:
      
      ```makefile
      build: ⚙️  # build the binary
      	go build -o $(BINARY) .
      
      apply: ⚙️ build  # apply config.yaml to the Claude Code config directory
      	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)
      ```
      
      - `build` rebuilds only when sources change (Make's normal rules apply)
      - Action targets invoke `./$(BINARY)` — the locally-built binary, not the one on `$PATH`.
        If needed, the user can override this rule if development is close to his system.
      
      ## Install target (Go)
      
      ```makefile
      install: ⚙️ build  # install the binary to PREFIX/bin (default: /usr/local/bin)
      	go install .
      	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && \
      	  echo "✅ Installed for all users" || echo "⚠️ System install failed"
      ```
      
      Install approach is usually: do local + try global
      - `go install` puts the binary in `$(GOPATH)/bin` (user-local)
      - `sudo install -m 0755` copies to `$(PREFIX)/bin` for system-wide availability
      - `|| echo …` degrades gracefully when `sudo` is unavailable
      
      ## Check target
      
      ```makefile
      check: ⚙️  # run static analysis and tests
      	go vet ./...
      	go test ./...
      
      check-fast: ⚙️  # fast local feedback loop
      	go test ./...
      
      test: ⚙️ check  # alias for check
      ```
      
      `make check` is the standard verification target used by development-flow docs.
      Always run `go vet` before `go test`; vet catches issues tests may not exercise.
      Keep `make test` as a compatibility alias when a repo already exposes it.
      Add `make check-fast` when full checks are slow; it should keep broad coverage but use cheap settings. Example: `trafficsim` runs one focused model via `MODEL=...`.
      
      ## Deployment target parity
      
      Any project with mutating provisioners (pushes a binary, config, or schedule to a remote host)
      must expose the same four self-documenting targets, so deploy/verify is never ad hoc SSH
      one-liners. For projects with remote deployment, install the optional
      `deployment-transparency` practice
      for why the query targets (`run`/`status`) must probe the live host rather than assume success
      from a completed `deploy`.
      
      ```makefile
      deploy: ⚙️ build  # deploy binary, configs, and cron schedules (DRY=1 for dry-run)
      	@scripts/deploy.sh $(if $(DRY),--dry-run)
      
      run: ⚙️  # query live deployment health (process, service, or job status)
      	@scripts/deploy.sh --status
      
      status: ⚙️ run  # alias for run
      
      backup: ⚙️  # sync state snapshots from the remote host
      	@scripts/backup.sh
      ```
      
      - `make deploy [DRY=1]` — ships binary, configs, and cron/systemd schedules; `DRY=1` must perform
        a real dry-run against the remote host, not a no-op.
      - `make run` / `make status` — read-only: query the actual remote process table, systemd units, or
        crontab, not the local repo state. Keep `status` as an alias when a repo already exposes `run`.
      - `make backup` — sync state snapshots (config overlays, data) down from the remote host before a
        risky deploy.
      - Every target here queries or mutates a real remote host — treat it like `make smoke` (see
        `@docs/AgenticLoop.md`): safe to define, but only run when you intend the live effect.

## Assistant

- `Bash.lite.md` — Defines concise Bash rules for strict mode, explicit `source`, `test` conditionals, quoting, cleanup, timeouts, directory scoping, and portable `awk`.
- `Bash.md` — Provides detailed Bash conventions covering scripting structure, conditionals, variables, output, traps, functions, portability, and safe command execution.
- `Cpp.md` — Recommends `ccache`, limited parallelism, and memory caps for resource-intensive C++/CGO builds.
- `GTK4.md` — Documents PyGObject/GTK4 patterns for application uniqueness, gesture handling, full-size Cairo overlays, and integration smoke tests.
- `Git.md` — Establishes default-branch workflows, proactive conventional commits, review expectations, repository hygiene, and restricted pushing.
- `Go.md` — Sets Go standards for dependencies, project structure, workspaces, spec-driven apps, Cobra CLIs, error handling, terminal widths, and table-driven tests.
- `Make.lite.md` — Summarizes Makefile conventions for help-first targets, sentinel-based phony rules, local builds, checks, and deployment parity.
- `Make.md` — Details self-documenting Makefiles using `⚙️`/`🤖` sentinels, dependency-driven workflows, local binaries, verification targets, and deployment commands.
- `ManPages.md` — Explains how Go/Cobra CLIs can generate, install, package, and keep roff man pages synchronized with command definitions.
- `Markdown.md` — Defines naming and content conventions for evergreen and ephemeral Markdown documents, plus hyperlink and diagram guidance.
- `Rust.md` — Recommends stable Rust tooling, minimal dependencies, `Result`-based error handling, library/CLI separation, and carefully documented unsafe code.
- `Zig.md` — Establishes explicit-allocation and performance practices while documenting Zig 0.16.0 API pitfalls for pipes, subprocesses, permissions, slices, environments, and C bindings.

## Activity

    $ /usr/bin/zsh -lc 'harnez stats --auto'
    status: Completed · exit 0
      TOOL  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      Bash  5      5.00       16434.3     -               -              0.0%
      rg    1      5.00       16137.0     -               -              0.0%
      sed   1      5.00       n/a         -               -              0.0%
      wc    1      1.00       16583.0     -               -              100.0%
      
      AGENT  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      codex  8      4.00       16404.6     -               -              12.5%
      
      PROJECT  CALLS  AVG SCORE  AVG TOKENS  SAVINGS TOKENS  SAVINGS BYTES  FAILURE RATE
      harnez   8      4.00       16404.6     -               -              12.5%
      
      compaction economics: insufficient_data (0 compaction(s))
        note: no compaction economics recorded
      distillation byte savings: no rows with distillation data
      
      heartbeat (harnez rate --ok, issue 179): none recorded
