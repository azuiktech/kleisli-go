# AGENTS.md

Instructions and protocols for AI coding assistants contributing to `kleisli-go`.

---

## 1. Mandatory Pre-PR Checklist

Before committing, pushing, or creating any Pull Request:

1. **Mandatory Linter Check**:
   Always run the linter and ensure **0 issues** are reported:
   ```bash
   golangci-lint run ./...
   ```
   If there are formatting or auto-fixable issues, run:
   ```bash
   golangci-lint run --fix ./...
   ```
   All remaining warnings or errors must be fixed before opening a PR. Never raise a PR with lint errors.
   - **ZERO LINTER SUPPRESSIONS**: Skipping, disabling, or suppressing any checks or linters (e.g. using `//nolint`, `//nolint:...`, or modifying linter configs to ignore errors) is strictly forbidden. If a linter, compiler, or analyzer reports a fault, the underlying code architecture and memory design must be fixed. No check may ever be bypassed.

2. **Mandatory Test Verification**:
   Execute the full test suite and confirm 100% clean pass:
   ```bash
   go test ./...
   ```

3. **Diff Scope Audit**:
   Inspect `git status` and `git diff` line-by-line. Never touch, stage, or delete untracked user files (such as review notes, local scripts, or documentation not part of the PR).

---

## 2. Development Lifecycle Protocol

1. **Step 0 — Issue Creation**:
   Create a tracking GitHub Issue for the feature or bugfix before writing code:
   ```bash
   gh issue create --title "..." --body "..."
   ```

2. **Step 1 — Types & Interfaces Checkpoint**:
   Define domain types, entities, and interfaces (no business logic implementation). Present them to the user for review.

3. **Step 2 — Test Suite Definition**:
   Write exhaustive Unit and Integration tests against the defined interfaces. Tests are immutable once written unless specifications change.

4. **Step 3 — Functional Implementation**:
   Implement minimal functional logic satisfying the interfaces.

5. **Step 4 & 5 — Verification & Review Checkpoint**:
   Run tests and linter locally, provide empirical proof, and present diffs to the user for explicit permission before committing.

6. **Step 6 — Commit, Push & PR**:
   Only after explicit user approval, commit minimal changes, push the branch, and open the Pull Request linked to the issue:
   ```bash
   gh pr create --title "..." --body "Closes #<issue_id>"
   ```

---

## 3. Core Functional Invariants

- **Functional Monadic Paradigm**:
  - Use `adt.Option[T]` and `adt.Result[T]` for error handling and fallible operations.
  - Eliminate nested `if err != nil` and `.Unwrap()` error checking; compose with `.Map`, `.FlatMap`, `.Recover`, `.Fold`.
  - Represent void outcomes with `adt.Void` / `adt.Unit`.
  - Avoid raw `for` loops where `stream.Stream` or `stream.Seq` pipelines can be used. This targets loops that hand-roll an *algorithm* a named combinator already provides — filter, map, find, reduce, group, transform-into-a-new-collection. A `for range` that merely executes a side effect per element (spawn a goroutine, cancel a future, call a handler) is a `ForEach`-shaped loop, not a reimplemented algorithm, and is not a violation on its own.
- **Minimal Footprint**: Keep diffs minimal, focused, and free of speculative abstractions or style-only refactoring.
