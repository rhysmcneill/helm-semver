# helm-semver
Semver release automation for Helm chart monorepos. Bumps chart versions from conventional commits (fix: → patch, feat: → minor, feat!: → major), packages and pushes to any OCI registry. Works as a GitHub Action or standalone Docker image on any CI platform.
