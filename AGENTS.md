# Repository guidance

## Pull request versioning

- Before opening a PR, identify its appropriate next SemVer version and state it in the PR description. Do not create a PR or release tag unless requested.
- Choose the highest required bump across all changes: major for breaking user-facing or compatibility changes, minor for backward-compatible capabilities or substantial dependency/architecture changes, and patch for bug fixes and other backward-compatible corrections.
- Dependency changes do not automatically require a version bump; judge their effect on Radikron. For this fix branch, removing `go-radiko` and `radigo` is treated as a minor update, while Windows-specific fixes are patch updates. Combined, target `v0.9.0` from the `v0.8.3` baseline.
- If the release baseline changes before PR creation, recalculate the target using the same rules rather than blindly retaining `v0.9.0`.
