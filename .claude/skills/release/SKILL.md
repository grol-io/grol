---
name: release
description: After a grol release is tagged/published in the GitHub UI, build the wasm release, sync it to the ../web-site repo, and propagate the new version to dependents (grol-discord-bot, fortio/fortio).
---

Run after the release (tag) has been created in the GitHub UI. Sister repos are expected next to this one (`../web-site`, `../grol-discord-bot`).

1. In this repo (grol): `git pull`, confirm the latest tag with `git tag --sort=-v:refname | head -1` and `gh release view --json tagName`.
2. `make wasm-release` (builds `wasm/grol.wasm` from `grol.io/grol/wasm@<tag>`).
3. In `../web-site`: `make` (target `sync` copies wasm/html/js/css from `../grol/wasm/` and shows `git diff`/`git status`).
4. Tour function tables (only change when builtins/help text changed): regenerate with `go run . -no-auto -quiet examples/functions_list.gr` and compare to the tables in `../web-site/tour/index.md` (from `| IO functions | |` to the line before "The above tables are automatically produced by"). Ignore the script's `--- ... ---` banner lines. If they differ, replace that block in the tour and include `tour/index.md` in the commit.
5. Commit in `../web-site` with the tag as the whole message, e.g. `v0.102.0` (past commits: `v0.101.4`, `v0.101.3, new go`; add a short note only if the release notably changes go version etc.), then push. The push triggers the GitHub Pages deployment; check the "pages build and deployment" run and that `https://grol.io/xterm.html` logs the new version.
6. grol-discord-bot (`../grol-discord-bot`): dependabot picks up new grol releases right away (the `cooldown` in its `dependabot.yml` excludes `grol.io/*` and `fortio.org/*`): click "Check for updates" (Insights > Dependency graph > Dependabot) or wait for the daily run. There is no API to trigger it. Alternatively make the PR by hand: `git checkout main && git pull`, branch `grol-<tag>`, `go get grol.io/grol@<tag> && go mod tidy && go build ./...`, commit "Bump grol.io/grol to <tag>", push, `gh pr create`. Merge after CI passes (check for `action_required` runs on fork PRs), then make a grol-discord-bot release in the UI (patch bump for a dependency-only update, notes mentioning the grol version and its fixes).
7. Deploy grol-discord-bot after its release; `@grol version` in Discord should show the new bot and grol versions.
8. Other dependents (e.g. fortio/fortio): the update PR must come from dependabot so it can be approved by the maintainer. Click "Check for updates" on the repo's Dependabot tab (`https://github.com/fortio/fortio/network/updates`) and look for the PR with `gh pr list --author "app/dependabot"`. Repos whose `dependabot.yml` doesn't exclude `grol.io/*` from `cooldown` only see the release after the default 3 day cooldown (new default since July 2026).
