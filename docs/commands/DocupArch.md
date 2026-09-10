# Docup Architecture

Maintain the target repository's architecture and design documentation. Treat
the target project as the source of truth; do not assume its language,
framework, runtime, deployment model, interface, or document layout.

Inspect these sources first, stopping at the Docup evidence cap:

1. `AGENTS.md` and applicable local instructions.
2. The repository's documentation index, README, and existing architecture or
   design documents.
3. Top-level manifests, build files, entry points, and deployment descriptors.
4. The smallest relevant set of implementation files and tests that establish
   the selected architecture claims.
5. At most 10 relevant recent commits when history is needed to confirm drift.

Discover the project's shape before choosing the review surface. Depending on
the repository, that may include a library API, CLI, desktop or TUI app, web
application, web service, worker, system service, data pipeline, plugin, or a
combination. Identify the real boundaries between components, users or
callers, storage, external services, processes, and deployment environments.

Review one architecture surface per invocation. The user may name a focus in
plain request text; otherwise choose the smallest clearly bounded surface from
the target project's own documentation and structure. Check that the selected
documentation agrees with the implementation about boundaries, responsibilities,
data or control flow, lifecycle, configuration, failure behavior, and runtime
assumptions. Check referenced paths, commands, links, and diagrams as part of
the same review.

Update the existing architecture or design document when possible. If the
project has no suitable document, create the smallest conventional document
indicated by its own documentation layout and link it from the relevant index.
Preserve the project's terminology and format. Do not introduce a generic
architecture taxonomy merely to fit this workflow.

Keep the work bounded:

- inspect at most 10 evidence files;
- edit at most 3 documentation files;
- change no code, configuration, issues, generated files, or installations;
- report a no-op when the selected surface is accurate;
- report remaining uncertainty when the project shape or evidence cap prevents
  a reliable conclusion.

Report the project shape inferred, architecture surface reviewed, evidence
inspected, concrete drift found, files changed, and checks actually run.
