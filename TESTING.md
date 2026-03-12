# Senate — Test Suite Assessment

Assessed against `/home/polis/tools/TEST_RUBRIC.md`.

---

## Rubric Scores

| Dimension                          | Before | After | Delta |
|------------------------------------|--------|-------|-------|
| 1. E2E Realism                     | 0      | 4     | +4    |
| 2. Unit Test Behaviour Focus       | 3      | 3     | 0     |
| 3. Edge Case & Error Path Coverage | 2      | 4     | +2    |
| 4. Test Isolation & Reliability    | 4      | 4     | 0     |
| 5. Regression Value                | 2      | 4     | +2    |
| **Total**                          | **11** | **19**| **+8**|

Grade: **B** (was D). The core `cmdAsk` orchestrator is now testable and tested.

---

## Assessment per Dimension

### 1. E2E Realism — 4/5

16 E2E tests exercise the compiled binary via `exec.Command`. The `cmdAsk` orchestrator — previously untestable — now uses an `Executor` interface with a test double that replaces only the external Claude CLI call. Tests exercise the full pipeline: question → case creation → Claude execution (via test double) → verdict parsing → storage → precedent indexing → stdout JSON output. Only a true E2E test with the real Claude binary is missing (infeasible without API credits).

### 2. Unit Test Behaviour Focus — 3/5

Tests target observable behaviour: `cmdFileCase` checks that a case ID is returned and the file is persisted, `TestVerdictPipelineRoundTrip` simulates the complete post-deliberation data flow, and handoff tests use a `fakeRunner` interface to verify skip/create logic without touching `br`. However, several tests still check exact string fragments in stdout (fragile), and a few test private helpers directly rather than through the public API.

### 3. Edge Case & Error Path Coverage — 4/5

Covers: validation rejects for all Case/Verdict fields, corrupt JSON on disk, missing files, deferred-verdict-skips-handoff, non-binding-verdict-skips-handoff, `br` CLI not found, whitespace/empty inputs, TMPDIR failure, bad store directory, verdict file missing after Claude exits, malformed verdict JSON from Claude output, Claude non-zero exit. Missing: concurrent store writes, disk-full/permission-denied mid-write, very large inputs, relay outbox append failures.

### 4. Test Isolation & Reliability — 4/5

All tests use `t.TempDir()` for filesystem isolation and `t.Setenv()` for environment overrides. No shared state, no sleep calls, no ordering dependencies. E2E tests build the binary once in `TestMain` (acceptable). The only concern is the `captureStdout`/`captureStderr` helpers that swap global `os.Stdout`/`os.Stderr`, which would break under `t.Parallel()`.

### 5. Regression Value — 4/5

`TestVerdictPipelineRoundTrip` is the highest-value test — it would catch breakage in the verdict JSON parsing, store persistence, precedent indexing, and search. The E2E roundtrip test catches binary compilation and CLI dispatch regressions. Handoff skip-logic tests catch the deferred/non-binding business rules. The `cmdAsk` orchestrator tests (via `fakeExecutor`) now cover the full main path: Claude invocation, verdict file reading, JSON parsing, verdict storage, precedent indexing, binding derivation, and stdout output. Breaking any of these would fail tests.

---

## What the Suite is MISSING

This is the most important section.

1. **`cmdStart` (interactive mode) has zero tests.** The entire interactive deliberation pathway is uncovered.

2. **Relay outbox writing is untested.** After storing a verdict, `cmdAsk` appends to a JSONL relay file. No test verifies this append, the file format, or error handling.

3. **Handoff `br` argument construction.** The `fakeRunner` captures args but no test asserts the exact flags/title/body passed to `br`. A subtle change to argument order would go undetected.

4. **Concurrent store writes.** The store uses atomic tmp+rename, but no test exercises this under contention.

5. **Search result ranking.** `scoreRecord` counts word occurrences, but no test verifies ordering when multiple precedents have varying relevance.

---

## Changelog

### 2026-02-28 — Agent: ares (pol-2u62)

- Added: `Executor` interface in `cli.go` — extracts the Claude CLI invocation behind a testable seam
- Added: `execExecutor` production implementation shells out to `claude` as before
- Added: `fakeExecutor` test double writes pre-configured verdict JSON to temp file
- Added: `TestCmdAsk_HappyPath` — valid case → valid verdict → stored + precedent indexed + stdout JSON
- Added: `TestCmdAsk_VerdictFileMissing` — claude exits ok but no verdict file → clear error
- Added: `TestCmdAsk_VerdictJSONMalformed` — invalid JSON in verdict file → error with context
- Added: `TestCmdAsk_ClaudeExitsNonZero` — executor returns error → surfaced clearly
- Added: `TestCmdAsk_HandoffTriggeredForBindingVerdict` — approved verdict stored as binding
- Added: `TestCmdAsk_HandoffSkippedForDeferredVerdict` — deferred verdict stored as non-binding
- Assessment: Moved from D (11/25) to B (19/25). The cmdAsk main path — previously the single biggest gap — now has full regression protection.

### 2026-02-28 — Agent: ares

- Added: `senate_e2e_test.go` — 16 E2E tests exercising the compiled binary (file-case, precedent, handoff, health, roundtrip)
- Added: `TestVerdictPipelineRoundTrip` — simulates the complete post-deliberation workflow (parse verdict JSON -> build Verdict -> store -> load -> index as precedent -> search). This is the single highest-value test in the suite.
- Added: `TestVerdictPipelineDeferredSkipsHandoff` — verifies the business rule that deferred verdicts are not binding and would not create handoff beads.
- Added: CLI tests for cmdHealth, cmdHandoff (deferred, non-binding, JSON, dispatcher), cmdAsk error paths, cmdFileCase bad-store-dir
- Added: Handoff tests for explicit-workspace enforcement, parseBeadID edge cases, and title format verification
- Added: Precedent tests for SearchRelevantPrecedent (type filter, empty keywords, limit), Record.Validate (all fields), LoadAll malformed lines, Add validation
- Added: Core validation tests for all Verdict.Validate rejection paths, Case edge cases (missing filed_at, blank evidence), Normalize defaults
- Added: Store tests for SaveCase/SaveVerdict validation errors, LoadCase/LoadVerdict corrupt JSON, path helpers
- Coverage delta: 66.3% -> 75.7% (meaningful: pipeline round-trip, E2E binary tests, all validation rejection paths, handoff business rules)
- Assessment: Moved from D (11/25) to C (16/25).
