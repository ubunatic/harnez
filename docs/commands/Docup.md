# Docup

Use this skill when the user explicitly invokes Docup, with or without a
category, or otherwise names a supported Docup category.

Supported categories: `testing`, `arch`, `cli`, `config`.

Interpret the request as plain text; do not depend on harness-specific argument
substitution or nested slash-command behavior. An explicit supported category
takes precedence. If the user explicitly requests multiple categories, ask them
to select one.

When no supported category is explicit, use Neus to infer one from the user's
focus and minimal repository context. For a bare invocation, first identify a
bounded candidate surface from the README, documentation index, or recent
relevant changes. Count inspected files toward the shared evidence cap.

Consult `neus --help` and `neus decide --help` for current usage, then use
`neus decide` to choose among `testing` (verification), `arch` (component
boundaries and lifecycle), `cli` (commands, flags, examples), and `config`
(settings, defaults, precedence). Use `neus ask` when repository evidence is
needed. Keep Neus command mechanics in its CLI help rather than this skill.

Verify the suggestion against the request and repository evidence; treat it as
advisory. If Neus fails or abstains but the evidence clearly identifies a
category, use that category and disclose the fallback. Otherwise ask the user
to choose from the supported categories before editing.

State the inferred category and bounded focus before making documentation
changes. Inferring a category does not authorize changes outside Docup's scope.

After selecting a category, read only its companion guide from the installed
skill's `references/` directory:

- `testing` → `references/DocupTesting.md`
- `arch` → `references/DocupArch.md`
- `cli` → `references/DocupCli.md`
- `config` → `references/DocupConfig.md`

Apply the selected guide's bounded workflow to the current repository. Keep one
repository and one category in scope, inspect at most 10 evidence files, and
make at most 3 documentation edits in one invocation. Preserve accurate text
and report a no-op when no evidenced update is needed. Do not change code,
create tickets, run global apply/init operations, or launch another skill as a
side effect.

Report the evidence inspected, concrete drift found, files changed, and checks
actually run. State any remaining uncertainty when the scope cap prevents a
complete review.
