# Repository guidance

## Versioning and releases

- Before opening a PR, determine its next SemVer version from the latest release (`gh release view --json tagName`) and state it in the PR description.
- Choose the highest required bump across all changes since that release: major for breaking user-facing or compatibility changes, minor for backward-compatible capabilities or substantial dependency/architecture changes, and patch for bug fixes and other backward-compatible corrections. Before `v1.0.0`, breaking changes bump the minor version.
- Dependency changes do not automatically require a version bump; judge their effect on Radikron.
- Bump the version in the PR itself when the target differs from the version in the repository: `info.version` in `cmd/radikron-gui/build/config.yml` and `version` in `cmd/radikron-gui/frontend/package.json`, then run `wails3 task common:update:build-assets` in `cmd/radikron-gui` to propagate it to `Info.plist` and the Windows metadata. The CLI version comes from the release tag through goreleaser and needs no file change.
- If an earlier unreleased PR already set the same target, keep it. If the latest release changed before PR creation, recalculate the target rather than keeping a stale one.
- After the PR is merged, create the release from the merge commit on `main` with `gh release create vX.Y.Z --target <merge-sha> --generate-notes`. Publishing the release creates the tag and starts the release workflow, which builds, signs, and uploads every asset. Never tag a PR branch: squash merges leave branch commits out of `main`.
