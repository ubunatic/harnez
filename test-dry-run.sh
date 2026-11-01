#!/bin/bash
set -e

HARNEZ=$PWD/harnez

DIR=$(mktemp -d)
echo "Setting up in $DIR"
cd $DIR

git init -q
git config user.name "Test"
git config user.email "test@example.com"
git config core.abbrev 7
git config merge.harnez-issues-index.driver true

mkdir issues
cat << 'INDEX' > issues/README.md
# Issues

- [1010](1010-base.md) — base
INDEX
cat << 'BASE' > issues/1010-base.md
# 1010 — base
BASE

echo 'issues/README.md merge=harnez-issues-index' > .gitattributes
git add issues .gitattributes
git commit -q -m "initial"

git branch -m main
git checkout -q -b feature

# Main gets 1011
git checkout -q main
cat << 'MAIN' > issues/1011-main.md
# 1011 — main
MAIN
cat << 'INDEX2' > issues/README.md
# Issues

- [1011](1011-main.md) — main
- [1010](1010-base.md) — base
INDEX2
git add issues
git commit -q -m "1011 main"

# Feature gets 1011 too
git checkout -q feature
cat << 'FEAT' > issues/1011-feat.md
# 1011 — feat
FEAT
cat << 'INDEX3' > issues/README.md
# Issues

- [1011](1011-feat.md) — feat
- [1010](1010-base.md) — base
INDEX3
git add issues
git commit -q -m "1011 feat"

echo "Running dry-run..."
$HARNEZ issues rebase --dry-run main
