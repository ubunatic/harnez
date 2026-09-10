# Docup

Use this skill only when the user explicitly invokes `/docup <category>` or
otherwise names a supported Docup category.

Supported category: `testing`.

Interpret the explicit category as plain request text. Accept exactly one
supported category. If the category is missing, unknown, or ambiguous, report
the supported categories and stop without editing files. Do not depend on
harness-specific argument substitution or nested slash-command behavior.

After selecting a category, read only its companion guide from the installed
skill's `references/` directory. For `testing`, read
`references/DocupTesting.md`.

Apply the selected guide's bounded workflow to the current repository. Keep one
repository and one category in scope, inspect at most 10 evidence files, and
make at most 3 documentation edits in one invocation. Preserve accurate text
and report a no-op when no evidenced update is needed. Do not change code,
create tickets, run global apply/init operations, or launch another skill as a
side effect.

Report the evidence inspected, concrete drift found, files changed, and checks
actually run. State any remaining uncertainty when the scope cap prevents a
complete review.
