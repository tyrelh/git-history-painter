# Build
```bash
go build ./cmd/git-history
```

# Usage
```bash
./git-history --dir ~/Projects/git-history
```

Existing tool-painted commits are loaded into the grid on startup, so re-running against the same repo lets you edit (including erase / lower intensity). Submitting rewrites the tool-managed history to match what you painted; non-tool commits are preserved.

Flags:
- `-dir <path>` — target git repo (default `.`)
- `-no-load` — start from an empty grid even if the repo already has painted commits
- `-dry-run` — paint but don't write anything

## Pushing changes
```bash
cd ~/Projects/git-history   # or wherever -dir pointed

# authenticate with gh
brew install gh
gh auth login              # browser flow
gh auth setup-git          # configures git to use gh as credential helper

git push --force-with-lease origin main
```
