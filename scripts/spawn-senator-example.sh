#!/bin/bash
# Example script showing how to spawn a senator agent for deliberation
# This demonstrates the architecture described in docs/ARCHITECTURE.md

set -euo pipefail

# Trap errors for better debugging
trap 'echo "Error at line $LINENO: $BASH_COMMAND" >&2' ERR

# Configuration
SENATE_ROOT="${SENATE_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
TMUX_SOCKET="/tmp/tmux-senate.sock"
SENATOR_NAME="${1:-pragmatist}"
CASE_ID="${2:-SNT-001}"
CONFIG_FILE="${SENATE_ROOT}/config/senate-config.json"

# Session naming
SESSION_NAME="senator-${SENATOR_NAME}-${CASE_ID}"
PROMPT_FILE="/tmp/senate-prompt-${SENATOR_NAME}-${CASE_ID}.md"
OUTPUT_DIR="${SENATE_ROOT}/output/${CASE_ID}"
MEMORY_DIR="${SENATE_ROOT}/memory/senators/${SENATOR_NAME}"

# Ensure directories exist
mkdir -p "${OUTPUT_DIR}"
mkdir -p "${MEMORY_DIR}/cases"

echo "=== Spawning Senator ${SENATOR_NAME} for case ${CASE_ID} ==="

# Step 1: Load senator configuration
# In production, this would parse the JSON config
SENATOR_MODEL="claude:opus"
TEMPLATE_FILE="${SENATE_ROOT}/templates/${SENATOR_NAME}.md"

# Step 2: Build system prompt with identity and memory
cat > "${PROMPT_FILE}" <<EOF
# Senator Session: ${CASE_ID}
# Loading identity and memory for ${SENATOR_NAME}

$(cat "${TEMPLATE_FILE}")

## Your Recent Memory

$(if [ -f "${MEMORY_DIR}/MEMORY.md" ]; then
    echo "### Long-term Memory"
    head -50 "${MEMORY_DIR}/MEMORY.md"
fi)

$(if [ -f "${MEMORY_DIR}/precedents.jsonl" ]; then
    echo "### Recent Precedents"
    # REASON: jq errors are expected if file is empty/malformed, fallback message is appropriate
    tail -10 "${MEMORY_DIR}/precedents.jsonl" | jq -r '.summary' 2>/dev/null || echo "No recent precedents"
fi)

## Session Instructions

You are now in session for case ${CASE_ID}. Wait for the case presentation, then provide your position using the structured format described above.
EOF

# Step 3: Create tmux session with Claude
echo "Creating tmux session: ${SESSION_NAME}"
tmux -S "${TMUX_SOCKET}" new-session -d -s "${SESSION_NAME}" \
    -c "${SENATE_ROOT}" \
    "claude -p --model opus --dangerously-skip-permissions --system-prompt-file '${PROMPT_FILE}'"

# Give Claude a moment to initialize
sleep 2

# Step 4: Send case presentation (would be loaded from case file)
echo "Sending case presentation..."
tmux -S "${TMUX_SOCKET}" send-keys -t "${SESSION_NAME}" "
=== CASE PRESENTATION START ===
Case ID: ${CASE_ID}
Type: rule_evolution
Filed: $(date -u +%Y-%m-%dT%H:%M:%SZ)

Summary: Should Truthsayer rule TS-042 be relaxed to reduce false positives?

Question: The async error handler pattern check (TS-042) has generated 15 false positives in the last week, all in legitimate error recovery code. Should we relax this rule to exclude async contexts, or maintain strict checking?

Evidence:
- 15 false positives logged in past 7 days
- All false positives involve async/await error handlers
- Developer friction score: 7/10 (high frustration)
- No actual bugs caught by this rule in async contexts
- 3 developers have disabled the rule locally

Requested Decision: Amend the rule to exclude async error handlers while maintaining checks in synchronous code.
=== CASE PRESENTATION END ===

Please provide your initial position on this case.
" Enter

# Step 5: Monitor for completion (simplified version)
echo "Waiting for senator response..."
echo "Session: tmux -S ${TMUX_SOCKET} attach -t ${SESSION_NAME}"
echo "Output will be captured to: ${OUTPUT_DIR}/${SENATOR_NAME}-initial.txt"

# In production, this would:
# - Poll for response completion markers
# - Capture output when complete
# - Parse structured response
# - Signal completion to orchestrator

# Example completion detection (simplified)
sleep 5  # In reality, would poll for markers

# Capture current output
tmux -S "${TMUX_SOCKET}" capture-pane -t "${SESSION_NAME}" -p > "${OUTPUT_DIR}/${SENATOR_NAME}-initial.txt"

echo "=== Senator ${SENATOR_NAME} spawned successfully ==="
echo "Session: ${SESSION_NAME}"
echo "Output: ${OUTPUT_DIR}/${SENATOR_NAME}-initial.txt"
echo "To interact: tmux -S ${TMUX_SOCKET} attach -t ${SESSION_NAME}"
echo "To cleanup: tmux -S ${TMUX_SOCKET} kill-session -t ${SESSION_NAME}"