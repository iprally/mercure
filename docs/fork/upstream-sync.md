# Updating This Fork from Upstream

This fork tracks `dunglas/mercure` while keeping IPRally-specific changes such as Redis transport, IAM support, history replay, workflow names, and fork-specific Docker/test configuration.

The `main` branch currently requires the merge queue and rejects merge commits. Because of that, upstream updates should be prepared in a pull request, and the final branch may need to be linear even when the conflict resolution was tested as a real merge.

## Procedure

1. Fetch both remotes:

   ```sh
   git fetch origin
   git fetch upstream
   ```

2. Start from the current fork `main` and create a backup branch:

   ```sh
   git checkout main
   git pull --ff-only origin main
   git branch backup/pre-upstream-sync-YYYYMMDD
   ```

3. Create a merge-resolution branch from the fork `main`:

   ```sh
   git checkout -b sync/upstream-main-YYYYMMDD origin/main
   ```

4. Merge upstream locally to expose the real conflict set:

   ```sh
   git merge upstream/main
   ```

5. Resolve conflicts with these defaults:
   - Keep upstream changes for generic project updates, dependency updates, generated files, and upstream CI/test improvements.
   - Preserve IPRally fork behavior for Redis transport, IAM support, history replay, renamed workflows, and fork-specific Docker/test configuration.
   - Adapt fork-specific code to upstream API changes instead of keeping old upstream-facing interfaces.

6. Run verification before preparing the PR:

   ```sh
   go test -timeout 300s -tags=deprecated_server,deprecated_transport,nobadger,nomysql,nopgx ./...
   (cd caddy && go test -timeout 300s -tags=deprecated_server,deprecated_transport,nobadger,nomysql,nopgx ./...)
   go run golang.org/x/vuln/cmd/govulncheck@latest ./...
   (cd caddy && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)
   (cd conformance-tests && npm audit --audit-level=high)
   ```

7. If `main` still rejects merge commits, convert the tested merge result into a linear PR branch:

   ```sh
   git checkout -b sync/upstream-main-YYYYMMDD-linear origin/main
   git checkout <tested-merge-commit> -- .
   git commit -m "Sync fork with upstream main"
   ```

8. Push the branch and open a PR to `main`. Let CI and the merge queue be the final gate.

## Maintenance Notes

The linear PR branch satisfies the current repository rules, but it does not preserve `upstream/main` as Git ancestry. Future upstream syncs may therefore need to repeat some conflict resolution because Git cannot see the previous PR as a true upstream merge.

For cleaner long-term upstream tracking, allow merge commits for dedicated upstream-sync PRs or maintain a protected integration branch that records upstream ancestry.
