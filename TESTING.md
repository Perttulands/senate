# Senate — Test Suite Assessment

Assessed against `/home/polis/tools/TEST_RUBRIC.md`.

---

## Rubric Scores

| Dimension                          | Before | After | Delta |
|------------------------------------|--------|-------|-------|
| 1. E2E Realism                     | 0      | 3     | +3    |
| 2. Unit Test Behaviour Focus       | 3      | 3     | 0     |
| 3. Edge Case & Error Path Coverage | 2      | 3     | +1    |
| 4. Test Isolation & Reliability    | 4      | 4     | 0     |
| 5. Regression Value                | 2      | 3     | +1    |
| **Total**                          | **11** | **16**| **+5**|

Grade: **C** (was D). Functional but insufficient for a city tool.

---

## Assessment per Dimension

### 1. E2E Realism — 3/5

16 E2E tests exercise the compiled binary via `exec.Command`: file-case (happy + error + all types), precedent search (text + JSON + empty + type filter), handoff (missing case + missing verdict), health, version/help, and a file-case-to-precedent roundtrip. However, the most important workflow — `senate ask` — cannot be tested E2E because it requires the Claude CLI binary. This single gap caps the score at 3.

### 2. Unit Test Behaviour Focus — 3/5

Tests target observable behaviour: `cmdFileCase` checks that a case ID is returned and the file is persisted, `TestVerdictPipelineRoundTrip` simulates the complete post-deliberation data flow, and handoff tests use a `fakeRunner` interface to verify skip/create logic without touching `br`. However, several tests still check exact string fragments in stdout (fragile), and a few test private helpers directly rather than through the public API.

### 3. Edge Case & Error Path Coverage — 3/5

Covers: validation rejects for all Case/Verdict fields, corrupt JSON on disk, missing files, deferred-verdict-skips-handoff, non-binding-verdict-skips-handoff, `br` CLI not found, whitespace/empty inputs, TMPDIR failure, bad store directory. Missing: concurrent store writes, disk-full/permission-denied mid-write, very large inputs, relay outbox append failures, malformed verdict JSON from Claude output.

### 4. Test Isolation & Reliability — 4/5

All tests use `t.TempDir()` for filesystem isolation and `t.Setenv()` for environment overrides. No shared state, no sleep calls, no ordering dependencies. E2E tests build the binary once in `TestMain` (acceptable). The only concern is the `captureStdout`/`captureStderr` helpers that swap global `os.Stdout`/`os.Stderr`, which would break under `t.Parallel()`.

### 5. Regression Value — 3/5

`TestVerdictPipelineRoundTrip` is the highest-value test — it would catch breakage in the verdict JSON parsing, store persistence, precedent indexing, and search. The E2E roundtrip test catches binary compilation and CLI dispatch regressions. Handoff skip-logic tests catch the deferred/non-binding business rules. But: the core `cmdAsk` function (the deliberation orchestrator) has zero coverage of its main path, meaning someone could break the Claude CLI invocation, verdict file reading, or transcript saving without any test failing.

---

## What the Suite is MISSING

This is the most important section.

1. **The `senate ask` workflow is untestable.** `cmdAsk` shells out to `claude` via `exec.Command`. Without the Claude CLI binary, the main codepath (~60% of cli.go) is unreachable. No mock or test double exists for this dependency. This is the single biggest gap — the tool's core value proposition has no automated regression protection.

2. **No mock for the Claude CLI call.** The handoff package solved this with a `Runner` interface. The CLI package has no equivalent — `cmdAsk` directly calls `exec.Command("claude", ...)`. Introducing a similar interface would unlock testing of verdict file reading, transcript construction, precedent indexing, and relay outbox writing.

3. **`cmdStart` (interactive mode) has zero tests.** The entire interactive deliberation pathway is uncovered.

4. **Relay outbox writing is untested.** After storing a verdict, `cmdAsk` appends to a JSONL relay file. No test verifies this append, the file format, or error handling.

5. **Verdict JSON parsing failures.** What happens when Claude writes malformed verdict JSON? The real-world failure mode (bad JSON from LLM output) has no test.

6. **Handoff `br` argument construction.** The `fakeRunner` captures args but no test asserts the exact flags/title/body passed to `br`. A subtle change to argument order would go undetected.

7. **Concurrent store writes.** The store uses atomic tmp+rename, but no test exercises this under contention.

8. **Search result ranking.** `scoreRecord` counts word occurrences, but no test verifies ordering when multiple precedents have varying relevance.

---

## Changelog

### 2026-02-28 — Agent: ares

- Added: `senate_e2e_test.go` — 16 E2E tests exercising the compiled binary (file-case, precedent, handoff, health, roundtrip)
- Added: `TestVerdictPipelineRoundTrip` — simulates the complete post-deliberation workflow (parse verdict JSON -> build Verdict -> store -> load -> index as precedent -> search). This is the single highest-value test in the suite.
- Added: `TestVerdictPipelineDeferredSkipsHandoff` — verifies the business rule that deferred verdicts are not binding and would not create handoff beads.
- Added: CLI tests for cmdHealth, cmdHandoff (deferred, non-binding, JSON, dispatcher), cmdAsk error paths, cmdFileCase bad-store-dir
- Added: Handoff tests for CreateBeadFromVerdict (br not found), defaultWorkspaceDir (env variants), parseBeadID edge cases, title format verification
- Added: Precedent tests for SearchRelevantPrecedent (type filter, empty keywords, limit), Record.Validate (all fields), LoadAll malformed lines, Add validation
- Added: Core validation tests for all Verdict.Validate rejection paths, Case edge cases (missing filed_at, blank evidence), Normalize defaults
- Added: Store tests for SaveCase/SaveVerdict validation errors, LoadCase/LoadVerdict corrupt JSON, path helpers
- Coverage delta: 66.3% -> 75.7% (meaningful: pipeline round-trip, E2E binary tests, all validation rejection paths, handoff business rules)
- Assessment: Moved from D (11/25) to C (16/25). Major remaining gap is the untestable `cmdAsk` main path.
