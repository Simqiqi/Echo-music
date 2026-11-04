# Releasing

1. Commit pending work, bring main up to date with origin, and run `go test ./internal/...`. Others push to main too, so a release cut from a stale main leaves out their work:

   ```bash
   git pull --rebase origin main
   ```

2. Bump `version` in `mygo.json`, and add a `## 0.1.8` section at the top of `CHANGELOG.md` saying, in Chinese, what changed for people using the app since the last release (`git log v0.1.7..HEAD`, after the pull). Commit both in a commit titled just the version, e.g. `0.1.8`. MyGo's updater shows that section in the app's update window, and the release page takes it as its notes; `mygo build` fails without one.
3. Push main first, and tag only once that push has gone through. Pushing both at once still pushes the tag when main is rejected, and the tag starts a release build of a commit that isn't on main:

   ```bash
   git push origin main
   git tag v0.1.8
   git push origin v0.1.8
   ```

   If main is rejected, pull again, add what came in to the changelog, amend the version commit, test, and push main again before tagging. If a tag went up on the wrong commit anyway, cancel its run, then delete the tag and the draft it made before tagging again:

   ```bash
   git push origin :refs/tags/v0.1.8
   gh release delete v0.1.8 --yes
   ```

4. `.github/workflows/release.yml` builds macOS, Windows and Linux and uploads them to a draft release. Wait for every job to pass:

   ```bash
   gh run watch "$(gh run list --workflow release.yml -L1 --json databaseId -q '.[0].databaseId')" --exit-status
   ```

   If a job fails, don't publish; fix it first.

5. Publish the draft and mark it latest:

   ```bash
   gh release edit v0.1.8 --draft=false --latest
   ```

6. Publishing the release also starts `.github/workflows/feed.yml`: it takes the release's update manifests, swaps the archives' addresses for the accelerator's prefix (`MIRROR` in the workflow, and `mirror` in `website/src/site.ts`), and pushes them to the `updates` branch. Installed apps check `https://cdn.jsdelivr.net/gh/Simqiqi/simqiqi-music@updates/update-<target>.json` — the release workflow bakes that feed into each build with `MYGO_LDFLAGS` — because GitHub's own is often unreachable in China. Only an address changes; the size and the signature of each archive are what `mygo build` made, so a mirror changes nothing an app trusts. If the accelerator dies, change `MIRROR` and run the workflow again by hand (`workflow_dispatch`, with the tag); the feeds of the released versions heal without a new build. Check both the feed and the release:

   ```bash
   curl -sL https://cdn.jsdelivr.net/gh/Simqiqi/simqiqi-music@updates/update-darwin-arm64.json
   curl -sL https://github.com/Simqiqi/simqiqi-music/releases/latest/download/update-darwin-arm64.json
   ```

   jsDelivr keeps a branch for twelve hours; the workflow purges its copy after pushing, so a feed that still shows the old version an hour later is worth a manual purge at `https://purge.jsdelivr.net/gh/Simqiqi/simqiqi-music@updates/update-darwin-arm64.json`.

7. Deploy the website. It reads the latest release when it's prerendered, so it has to be rebuilt after publishing:

   ```bash
   cd website && bun run deploy
   ```

   Then check https://echo-cloudmusic.<your-subdomain>.workers.dev/download/ shows the new version.

## The speech recognition libraries

Podcast captions run sherpa-onnx, whose libraries the app downloads with the model from the `sherpa-onnx-<version>` release, not from the app's own. They only change with sherpa-onnx's version, in `scripts/sherpa`. Then pack them again, signing the macOS ones with the app's Developer ID (the app's hardened runtime loads no others), commit the `internal/speech/natives.go` it writes, and upload them as a prerelease, which never becomes the latest that updates look for:

```bash
go run ./scripts/sherpa -sign "Developer ID Application: xinyu zeng (GJE9R5VE87)"
gh release create sherpa-onnx-2.0.1 --prerelease --latest=false build/sherpa/*
```
