# Docup Configuration

Maintain the repository's configuration documentation for one subsystem or
closely related group of settings. The user may name the focus; otherwise choose
the smallest clearly bounded surface with evidence of documentation drift.

Inspect these sources first, stopping at the Docup evidence cap:

1. `AGENTS.md` and applicable local instructions.
2. Existing configuration guides, documented examples, and their index links.
3. Authoritative schemas, embedded specifications, or default configuration files.
4. Relevant loading, validation, merge, and environment or CLI override logic.
5. Focused tests that establish defaults, precedence, or invalid-value behavior.

Check setting names, types, accepted values, units, defaults, required fields,
file locations, discovery order, and precedence against the actual sources.
Where relevant, check environment-variable mappings, relative-path resolution,
reload or restart requirements, and unknown or invalid setting behavior. Do not
infer precedence from schema order or treat sample values as runtime defaults.
If sources disagree, document only what the evidence establishes and report the
unresolved discrepancy without changing implementation.

Update existing configuration documentation and directly related snippets or
links. Keep examples minimal and use placeholders for secrets. Follow the
repository's documentation layout when a small new guide is justified. Edit
maintained documentation sources; do not modify schemas, runtime settings,
sample configuration files, generated references, or installations.

Verify changed paths, links, and formatting with the narrowest relevant checks.
Validate documentation snippets against an existing schema or validator when
available without applying settings or contacting live services. Distinguish
checks run from behavior established by inspected definitions and tests.

Inspect at most 10 evidence files and edit at most 3 documentation files. Create
no tickets and change no code. Report the settings surface reviewed, evidence
inspected, concrete drift, files changed, checks run, and remaining uncertainty.
Report a no-op when the selected documentation is accurate.
