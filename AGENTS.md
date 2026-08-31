# Repository Guidelines & Development Strategy

## 1. Branching Model

- **Base Branches:**
  - `main`: Latest stable or beta release. Tagged with semantic versioning (e.g. `v0.2.0`).
  - `develop`: Integration branch for active ongoing development.

- **Branch Naming and Scope (Strict Separation):**
  - Features: `feature/<milestone-or-feature-name>` (cut from `develop`, merged to `develop`)
  - Bugfixes: `fix/<issue-summary>` (cut from `develop`, merged to `develop`)
  - Refactors: `refactor/<component-name>` (cut from `develop`, merged to `develop`)
  - Hotfixes: `hotfix/<issue-summary>` (cut from `main`, merged to `main` and `develop`)
  - Releases: `release/<version>` (cut from `develop`, merged to `main` and `develop`)

- **Strict Isolation Rule:**
  Never combine feature development, bug fixing, and refactoring into a single branch or commit. Each branch must focus exclusively on its designated scope.

## 2. Commit and Verification Gates

1. **Pre-Commit Verification:**
   - Execute `go test -v ./...` before committing.
   - All tests must pass with exit code 0.
   - No untracked temporary files or broken imports.

2. **Commit Message Format (Conventional Commits):**
   - Format: `<type>(<scope>): <concise description>`
   - Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
   - Examples:
     - `feat(ws): implement centralized websocket broadcast hub`
     - `fix(config): correct properties file path resolution`
     - `refactor(minecraft): decouple log broadcasting via listeners`
     - `docs(repo): define branching strategy and commit guidelines in AGENTS.md`

3. **Release Procedure:**
   - When a milestone is ready, merge `develop` into `main`.
   - Tag `main` with the semantic version: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`.
   - Push tags with `git push origin --tags`.
