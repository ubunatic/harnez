# Docup CLI

Maintain the repository's documentation for one command or closely related
command group. The user may name the focus; otherwise choose the smallest
clearly bounded surface with evidence of documentation drift.

Inspect these sources first, stopping at the Docup evidence cap:

1. `AGENTS.md` and applicable local instructions.
2. The README, documentation index, and existing guide for the selected command.
3. Command registration, argument and flag definitions, and relevant handlers.
4. Focused tests or examples that establish the documented behavior.
5. Help-generation definitions or relevant recent commits when needed to resolve
   a specific discrepancy.

Compare documentation with the command's actual name, aliases, subcommands,
arguments, flags, defaults, scope, output, exit behavior, and side effects.
Check examples against the implementation, including required prerequisites and
whether commands read or modify local or external state. Keep configuration
internals outside this review unless needed to explain the selected command.

Update the existing guide and directly related examples or links. If no suitable
guide exists, follow the repository's documentation layout for the smallest
useful addition. Edit maintained sources rather than generated help or man pages;
report generated artifacts that require a separate refresh.

Verify changed paths, links, and formatting with the narrowest relevant checks.
Use read-only help or version probes when useful and known to be safe; do not
execute mutating examples merely to validate their wording. Distinguish behavior
established from definitions or tests from commands actually run.

Inspect at most 10 evidence files and edit at most 3 documentation files. Change
no code, configuration, issues, generated files, or installations. Report the
command surface reviewed, evidence inspected, concrete drift, files changed,
checks run, and remaining uncertainty. Report a no-op when the selected
documentation is accurate.
