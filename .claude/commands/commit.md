Commit Code in Staging

1. Check syntax and quality
   - Run a linter/formatter (eslint, prettier, flake8, etc.) depending on the project.
   - Ensure no syntax errors or obvious bad practices remain.
2. Format and organize code
   - Apply consistent code style.
   - Remove unused imports, variables, and dead code.
3. Commit changes
   - Remove claude signature from all commit messages
   - Use imperative mood in commit messages.
   - Be specific, concise, and ≤ 3 bullets.
   - Split into multiple commits if changes are substantial (e.g., one for bug fix, one for refactor).
   - Example:

```bash
git commit -m "Fix validation error in login form"
git commit -m "Refactor user service for better readability"
```
