# File a Repository Issue

## TL;DR

In a harnez-managed repository, run:

```bash
harnez find -d <repo> issues "<search terms>"
harnez issues new -d <repo> "<title>"
# Fill the returned ticket path using the repository's issue conventions.
harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"
```

Keep this workflow deliberately thin: the repository instructions and issue-tracking
documentation should already be in context. Inspect and follow them rather than
restating their full policy here.

Before drafting, search for duplicates and inspect nearby tickets for local conventions.
Choose priority independently from severity. Capture the concrete problem, scope,
acceptance criteria, and verification guidance; for exploratory requests, record
uncertainties instead of inventing implementation details.

Stage only the new ticket and regenerated index so unrelated working-tree changes remain
untouched. If the repository is not harnez-managed, follow its local tracker and commit
rules instead of assuming `harnez`, `issues/`, or an immediate tracker commit.
