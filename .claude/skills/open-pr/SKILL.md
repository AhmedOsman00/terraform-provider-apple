---
name: open-pr
description: Push the current branch and open a GitHub pull request against main with a conventional title and a short description of what's included. Use when the user asks to open, create, or raise a PR.
---

# Open PR

## Steps

1. Make sure you're not on `main`. If you are, use the `branch` skill first.
2. Check for uncommitted changes; if any belong in the PR, use the `commit` skill first.
3. Gather what's in the PR:
   ```bash
   git fetch origin main
   git log --oneline origin/main..HEAD
   git diff --stat origin/main...HEAD
   ```
4. Push: `git push -u origin HEAD`.
5. Create the PR with `gh pr create --base main --title ... --body-file -` using a heredoc.
6. Print the PR URL.

## Title

Same format as the commit skill: `<type>(<scope>): <summary>`. For a single-commit PR, reuse the commit subject. Otherwise summarise the whole branch.

## Description

Keep it small. A one-line summary, then a short bullet list of what's included:

```markdown
<One sentence on what this PR does and why.>

## What's included
- <change, in plain words>
- <change>
- <change>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

- 2–6 bullets; group related commits into one bullet rather than listing every commit.
- Describe behaviour, not file names, unless the file *is* the point (a workflow, a migration).
- Add a `## Notes` line only when the reviewer needs to know something: a migration to run, a flag to flip, something untested.
- End with the PR attribution line from the current attribution reminder, if one is present.

## Rules

- Don't open as draft, add reviewers, or add labels unless asked.
- If a PR for this branch already exists (`gh pr view`), show its URL instead of creating another.
