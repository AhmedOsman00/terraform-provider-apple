---
name: branch-commit
description: Move the current changes onto a new branch cut from main, then commit them. Use when the user asks to branch and commit, put changes on a new branch, or start a branch for the work just done.
---

# Branch and commit

Runs the `branch` and `commit` skills in sequence so the work in progress lands as one commit on a fresh branch off `main`.

## Steps

1. Run `git status` and `git diff` to see the changes. If there are none, stop and say so.
2. Create the branch with the `branch` skill. Base it on `origin/main` and pick the prefix and slug from what the changes do. Uncommitted changes carry over with `git switch -c <prefix>/<slug> origin/main`. If the switch refuses because a file differs between HEAD and `origin/main`, run `git stash`, create the branch, then `git stash pop`. If the pop conflicts, stop and show the conflict; don't resolve it unasked.
3. Check `git status` again to confirm the changes are on the new branch and nothing was lost.
4. Commit with the `commit` skill. Stage only the files that belong to the change.
5. Report the branch name and the commit summary. Don't push; the `open-pr` skill does that.
