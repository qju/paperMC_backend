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

## 3. Database Migration Governance

- **Strict Migration Requirement:**
  Direct un-versioned schema mutations (such as raw un-tracked `CREATE TABLE` or `ALTER TABLE` statements in store setup) are strictly prohibited.
- **Migration Engine Registration:**
  Any schema addition, modification, or data migration must be registered as an incremental version step in [`internal/database/migrations.go`](file:///home/marcin/Development/paperMC_backend/internal/database/migrations.go).
- **Sequential Versioning:**
  Every migration must have a strictly incremented integer `Version` (e.g. Version 1, Version 2) and execute inside the provided `*sql.Tx` transaction.
- **Backward Compatibility:**
  Migrations must be non-destructive and backward-compatible with existing production database files.

## 4. Documentation & Plan Synchronization

- **Mandatory README Update:**
  Always update [`README.md`](file:///home/marcin/Development/paperMC_backend/README.md) after completing commits so that documentation, endpoints, configuration flags, and architectural summaries accurately reflect the current state of the codebase.
- **Mandatory Plan Update:**
  Always update the plan file [`dev_plan.md`](file:///home/marcin/Development/paperMC_backend/dev_plan.md) after completing commits, checking off completed tasks and updating milestone statuses.

## 5. Mandatory Testing & Test Review Standards

- **Mandatory Tests with Every Addition:**
  Every new feature, endpoint, database operation, utility function, bug fix, or refactor must include comprehensive automated unit and/or integration tests. No code addition is complete without accompanying tests covering both happy paths and edge/error cases.
- **Mandatory Test Review on Code Changes:**
  Whenever existing code is modified or refactored:
  1. Review and update corresponding existing test suites to prevent test decay or stale assertions.
  2. Add regression tests specifically reproducing fixed bugs.
  3. Verify that changes do not decrease statement coverage across the affected packages.
  4. Ensure all package tests run deterministically and fast with zero race conditions or unclosed resources.
- **Target Coverage:**
  Maintain a minimum of 80% statement test coverage across all core internal backend packages (`internal/auth`, `internal/api`, `internal/config`, `internal/database`, `internal/minecraft`, `internal/updater`).


