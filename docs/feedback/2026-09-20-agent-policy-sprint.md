# Agent Policy Sprint Feedback

- The independent review caught two integration risks that unit-level happy paths missed: persistent policy shadowing and relative session directories.
- Treating `--persist` as a migration between managed targets produced clearer effective-state semantics than retaining duplicate blocks.
- Quota-1 remained workable because every repeat suite run followed a source or test correction; narrow read-only review avoided redundant verification.
- External `harnez agent start` failed without a diagnostic, so the sprint used native delegated sessions and recorded the harness failure immediately.
