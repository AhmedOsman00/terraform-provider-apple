---
name: commit
description: Commit staged or working-tree changes with a Conventional Commits message like "fix(bundle): keep the configured platform on import". Use when the user asks to commit, make a commit, or save changes to git.
---

# Commit

Write one commit in the form:

```
<type>(<scope>): <summary>
```

Example: `fix(bundle): keep the configured platform on import`

## Steps

1. Run `git status` and `git diff --staged` (and `git diff` if nothing is staged) to see what is going in.
2. If nothing is staged, stage only the files that belong to this change. Never `git add -A` blindly; leave unrelated work alone and ask if it's unclear what belongs together.
3. If the changes cover unrelated concerns, propose splitting them into separate commits.
4. Write the message, show it, and commit.

## Type

| type | use for |
|------|---------|
| `feat` | a new user-facing capability |
| `fix` | a bug fix |
| `refactor` | code change with no behaviour change |
| `perf` | performance improvement |
| `test` | adding or fixing tests only |
| `docs` | docs/, README, release notes |
| `ci` | .github/workflows, build scripts |
| `build` | `go.mod`/`go.sum`, Go toolchain, Makefile, GoReleaser configs |
| `chore` | config, tooling, linter settings, anything else |
| `revert` | reverting a previous commit |

`docs/` is generated: a schema or `MarkdownDescription` change that regenerates it is typed by the schema change (`feat`/`fix`), not `docs`. Use `docs` for `templates/`, `README.md`, `CLAUDE.md`, `ACCEPTANCE_TESTING.md` and the changelogs.

## Scope

One lowercase word naming the area touched. Prefer:

- the provider package under `internal/provider/`: `app`, `beta`, `bundle`, `certificate`, `device`, `inapppurchase`, `media`, `merchant`, `passtypeid`, `profile`, `subscription`, `territory`, `user`
- `provider` for `internal/provider/provider.go` and cross-package provider changes
- `client` for `internal/apple/` (the App Store Connect API client)
- `cli` for `cmd/apple` and `internal/cli/`, `export` for `internal/export/`
- top-level areas: `examples`, `templates`, `scripts`, `ci`, `release`
- a more specific noun when it reads better (`registry`, `signing`, `testflight`, `uploads`)

If a change to `internal/apple/` exists only to serve one provider package, use that package's scope: `fix(app): ...` for an availability client fix.

If the change spans many areas with no clear owner, omit the scope: `chore: ...`.

## Summary

- Lowercase, imperative mood ("add", "publish", "keep"), no trailing period.
- Say what changes, not how; aim for under ~72 characters total.
- Add `!` after the scope for breaking changes: `feat(app)!: ...`. Renaming a resource type or removing or narrowing a schema attribute is breaking.

## Body (optional)

Add a body after a blank line only when the *why* isn't obvious from the summary. Wrap at 72 columns.

## Rules

- End the message with the co-author trailer from the current attribution reminder, if one is present.
- Pass the message with a heredoc so formatting survives:
  ```bash
  git commit -F - <<'MSG'
  fix(app): create availability when apple answers 404

  Co-Authored-By: ...
  MSG
  ```
- If a schema, example or template changed, run `make generate` and include the regenerated `docs/` in the same commit; CI fails on a docs diff.
- Don't amend, push, or skip hooks unless asked.
