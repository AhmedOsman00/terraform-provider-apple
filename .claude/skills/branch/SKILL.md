---
name: branch
description: Create a git branch named feature/, bugfix/, or hotfix/ followed by a short kebab-case slug. Use when the user asks to create, start, or switch to a new branch.
---

# Branch

Branch names take the form:

```
<prefix>/<short-kebab-slug>
```

| prefix | use for | branch from |
|--------|---------|-------------|
| `feature/` | new functionality or improvements | `main` |
| `bugfix/` | a non-urgent bug fix | `main` |
| `hotfix/` | an urgent fix for a released provider (`v*`) or CLI (`cli-v*`) version | `main` (or the release tag being patched, if asked) |

Examples: `feature/app-preview-sets`, `bugfix/app-availability-404`, `hotfix/profile-import-reissue`

## Steps

1. Pick the prefix from the user's description. If it's ambiguous between bugfix and hotfix, ask whether it needs to ship urgently.
2. Build the slug: lowercase, words joined with `-`, 2–5 words, no ticket noise or dates.
3. Check `git status`. Uncommitted changes carry over to the new branch; mention that if any exist rather than stashing them unasked.
4. Update the base and create the branch:
   ```bash
   git fetch origin main
   git switch -c <prefix>/<slug> origin/main
   ```
   If the user wants to branch from the current HEAD instead (e.g. to carry work in progress), use `git switch -c <prefix>/<slug>`.
5. Confirm the name doesn't already exist locally or on origin (`git branch -a --list '*<slug>*'`) before creating.
6. Don't push unless asked; the `open-pr` skill pushes with `-u`.

Don't use other prefixes. Dependabot opens its own `dependabot/*` branches; don't create those by hand.
