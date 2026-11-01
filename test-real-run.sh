#!/bin/bash
set -e

make build >/dev/null
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

| # | File | Title | Status |
|---|------|-------|--------|
| 1010 | [1010-base.md](1010-base.md) | base | Open |
INDEX
cat << 'BASE' > issues/1010-base.md
# 1010 — base
**Status**: Open
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
**Status**: Open
MAIN
cat << 'INDEX2' > issues/README.md
# Issues

| # | File | Title | Status |
|---|------|-------|--------|
| 1011 | [1011-main.md](1011-main.md) | main | Open |
| 1010 | [1010-base.md](1010-base.md) | base | Open |
INDEX2
git add issues
git commit -q -m "1011 main"

# Feature gets 1011 too
git checkout -q feature
cat << 'FEAT' > issues/1011-feat.md
# 1011 — feat
**Status**: Open
FEAT
cat << 'INDEX3' > issues/README.md
# Issues

| # | File | Title | Status |
|---|------|-------|--------|
| 1011 | [1011-feat.md](1011-feat.md) | feat | Open |
| 1010 | [1010-base.md](1010-base.md) | base | Open |
INDEX3
git add issues
git commit -q -m "1011 feat"

echo "Running real rebase..."
$HARNEZ issues rebase main
git log --oneline
