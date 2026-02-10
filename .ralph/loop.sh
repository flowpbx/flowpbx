#!/bin/bash
set -e

# Ralph Loop - Orchestration script for the Ralph Wiggum Method
# Usage:
#   ./loop.sh              # Build mode, unlimited iterations
#   ./loop.sh plan         # Planning mode
#   ./loop.sh plan 5       # Planning mode, max 5 iterations
#   ./loop.sh build 10     # Build mode, max 10 iterations
#   ./loop.sh build 0 3    # Build mode, start at sprint 3

MODE="${1:-build}"
MAX_ITERATIONS="${2:-0}"
START_SPRINT="${3:-}"
ITERATION=0
OUTPUT_FILE=$(mktemp)
trap "rm -f $OUTPUT_FILE" EXIT

# Script directory (where .ralph lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SPRINTS_DIR="$SCRIPT_DIR/sprints"
CURRENT_SPRINT_FILE="$SCRIPT_DIR/CURRENT_SPRINT"

# Change to project directory
cd "$PROJECT_DIR"

# Validate mode
if [[ "$MODE" != "plan" && "$MODE" != "build" ]]; then
    echo "Error: Invalid mode '$MODE'. Use 'plan' or 'build'"
    exit 1
fi

PROMPT_FILE="$SCRIPT_DIR/PROMPT_${MODE}.md"
LEARNINGS_FILE="$SCRIPT_DIR/PROMPT_learnings.md"

if [[ ! -f "$PROMPT_FILE" ]]; then
    echo "Error: $PROMPT_FILE not found"
    exit 1
fi

# Create learnings file if it doesn't exist
if [[ ! -f "$LEARNINGS_FILE" ]]; then
    cat > "$LEARNINGS_FILE" << 'EOF'
# Learnings

Runtime discoveries and gotchas captured during build iterations.

## Project Setup
<!-- Package manager, build tools, etc. -->

## Code Patterns
<!-- Naming conventions, architectural decisions -->

## Common Pitfalls
<!-- Things that failed and how to avoid them -->
EOF
fi

# Function to find current sprint (first with incomplete tasks)
find_current_sprint() {
    for i in $(seq -w 1 30); do
        sprint_file="$SPRINTS_DIR/sprint-$i.md"
        if [[ -f "$sprint_file" ]]; then
            # Check if there are incomplete tasks
            if grep -q '^\- \[ \]' "$sprint_file" 2>/dev/null; then
                echo "$i"
                return
            fi
        fi
    done
    echo "done"
}

# Function to count tasks in sprint
count_sprint_tasks() {
    local sprint_file="$1"
    local total=$(grep -c '^\- \[' "$sprint_file" 2>/dev/null || echo "0")
    local done=$(grep -c '^\- \[x\]' "$sprint_file" 2>/dev/null || echo "0")
    echo "$done/$total"
}

# Initialize or set current sprint
if [[ -n "$START_SPRINT" ]]; then
    echo "$START_SPRINT" > "$CURRENT_SPRINT_FILE"
elif [[ ! -f "$CURRENT_SPRINT_FILE" ]]; then
    find_current_sprint > "$CURRENT_SPRINT_FILE"
fi

CURRENT_SPRINT=$(cat "$CURRENT_SPRINT_FILE")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color
BOLD='\033[1m'

echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Ralph Loop${NC} - $(basename "$PROJECT_DIR")"
echo -e "  Mode: ${CYAN}${MODE}${NC}"
if [[ "$CURRENT_SPRINT" != "done" ]]; then
    SPRINT_FILE="$SPRINTS_DIR/sprint-$CURRENT_SPRINT.md"
    SPRINT_NAME=$(head -1 "$SPRINT_FILE" | sed 's/^# //')
    TASK_COUNT=$(count_sprint_tasks "$SPRINT_FILE")
    echo -e "  Sprint: ${MAGENTA}$CURRENT_SPRINT${NC} - $SPRINT_NAME"
    echo -e "  Progress: ${GREEN}$TASK_COUNT${NC} tasks"
fi
[[ $MAX_ITERATIONS -gt 0 ]] && echo -e "  Max iterations: ${YELLOW}$MAX_ITERATIONS${NC}"
echo -e "${BOLD}================================================${NC}"

# Check if all sprints are done
if [[ "$CURRENT_SPRINT" == "done" ]]; then
    echo ""
    echo -e "${GREEN}${BOLD}All sprints complete!${NC}"
    exit 0
fi

while :; do
    ((ITERATION++))

    # Update current sprint (may have changed)
    CURRENT_SPRINT=$(find_current_sprint)
    echo "$CURRENT_SPRINT" > "$CURRENT_SPRINT_FILE"

    if [[ "$CURRENT_SPRINT" == "done" ]]; then
        echo ""
        echo -e "${GREEN}${BOLD}================================================${NC}"
        echo -e "${GREEN}${BOLD}  ✓ ALL SPRINTS COMPLETE!${NC}"
        echo -e "${GREEN}${BOLD}================================================${NC}"
        break
    fi

    SPRINT_FILE="$SPRINTS_DIR/sprint-$CURRENT_SPRINT.md"
    SPRINT_NAME=$(head -1 "$SPRINT_FILE" | sed 's/^# //')
    TASK_COUNT=$(count_sprint_tasks "$SPRINT_FILE")

    echo ""
    echo -e "${BOLD}━━━ Iteration $ITERATION ${GRAY}$(date '+%H:%M:%S')${NC} ${BOLD}━━━${NC}"
    echo -e "Sprint ${MAGENTA}$CURRENT_SPRINT${NC}: $SPRINT_NAME ${GREEN}[$TASK_COUNT]${NC}"
    echo ""

    # Clear output file for this iteration
    > "$OUTPUT_FILE"

    # Build the prompt (main prompt + learnings)
    FULL_PROMPT=$(cat "$PROMPT_FILE"; echo ""; echo "---"; echo ""; echo "Current Sprint: $CURRENT_SPRINT"; echo ""; cat "$LEARNINGS_FILE")

    # Run Claude and parse the JSON stream
    RAW_OUTPUT=$(mktemp)

    echo "$FULL_PROMPT" | claude -p \
        --dangerously-skip-permissions \
        --output-format=stream-json \
        --verbose \
        --model opus \
        2>&1 | tee "$RAW_OUTPUT" | while IFS= read -r line; do

        # Skip empty lines
        [[ -z "$line" ]] && continue

        # Try to parse as JSON
        if echo "$line" | jq -e '.' >/dev/null 2>&1; then
            TYPE=$(echo "$line" | jq -r '.type // empty')

            case "$TYPE" in
                system)
                    SUBTYPE=$(echo "$line" | jq -r '.subtype // empty')
                    if [[ "$SUBTYPE" == "init" ]]; then
                        MODEL=$(echo "$line" | jq -r '.model // "unknown"')
                        echo -e "${GRAY}[init] model: $MODEL${NC}"
                    fi
                    ;;

                assistant)
                    # Extract content from message
                    echo "$line" | jq -r '
                        .message.content[]? |
                        if .type == "text" then
                            .text
                        elif .type == "thinking" then
                            "[thinking] " + (.thinking | .[0:200]) + "..."
                        elif .type == "tool_use" then
                            "[tool] " + .name + ": " + (.input | tostring | .[0:80]) + "..."
                        else
                            empty
                        end
                    ' 2>/dev/null | while IFS= read -r content; do
                        if [[ "$content" == "[thinking]"* ]]; then
                            echo -e "${GRAY}$content${NC}"
                        elif [[ "$content" == "[tool]"* ]]; then
                            echo -e "${YELLOW}$content${NC}"
                        elif [[ -n "$content" ]]; then
                            echo "$content"
                        fi
                    done
                    ;;

                user)
                    # Show tool results briefly
                    RESULT=$(echo "$line" | jq -r '
                        .message.content[]? |
                        if .type == "tool_result" then
                            if .is_error == true then
                                "ERROR: " + (.content | tostring | .[0:150])
                            else
                                "→ " + (.content | tostring | .[0:100])
                            end
                        else
                            empty
                        end
                    ' 2>/dev/null)
                    if [[ -n "$RESULT" ]]; then
                        if [[ "$RESULT" == "ERROR:"* ]]; then
                            echo -e "${RED}$RESULT${NC}"
                        else
                            echo -e "${GRAY}$RESULT${NC}"
                        fi
                    fi
                    ;;

                result)
                    # Final result
                    COST=$(echo "$line" | jq -r '.cost_usd // 0')
                    DURATION=$(echo "$line" | jq -r '.duration_ms // 0')
                    DURATION_SEC=$(echo "scale=1; $DURATION / 1000" | bc 2>/dev/null || echo "?")
                    echo ""
                    echo -e "${GREEN}✓ Completed${NC} ${GRAY}(${DURATION_SEC}s, \$${COST})${NC}"
                    ;;
            esac
        else
            # Not JSON, print as-is (stderr or other output)
            echo -e "${GRAY}$line${NC}"
        fi
    done || true  # Don't fail on pipe errors

    # Extract text content from raw output for DONE detection
    jq -r 'select(.type == "assistant") | .message.content[]? | select(.type == "text") | .text' "$RAW_OUTPUT" 2>/dev/null > "$OUTPUT_FILE" || true
    rm -f "$RAW_OUTPUT"

    EXIT_CODE=0

    # Check for completion signal in output
    if grep -q "<promise>DONE</promise>" "$OUTPUT_FILE" 2>/dev/null; then
        echo ""
        echo -e "${GREEN}${BOLD}================================================${NC}"
        echo -e "${GREEN}${BOLD}  ✓ DONE - All sprints complete!${NC}"
        echo -e "${GREEN}${BOLD}================================================${NC}"
        break
    fi

    if [[ $EXIT_CODE -ne 0 ]]; then
        echo ""
        echo -e "${YELLOW}Claude exited with code $EXIT_CODE, retrying in 2s...${NC}"
        sleep 2
    fi

    # Check if we've reached max iterations
    if [[ $MAX_ITERATIONS -gt 0 && $ITERATION -ge $MAX_ITERATIONS ]]; then
        echo ""
        echo -e "${YELLOW}${BOLD}================================================${NC}"
        echo -e "${YELLOW}  Reached max iterations ($MAX_ITERATIONS)${NC}"
        echo -e "${YELLOW}${BOLD}================================================${NC}"
        break
    fi

    # Brief pause between iterations
    sleep 1
done

echo ""
echo -e "Loop completed after ${BOLD}$ITERATION${NC} iteration(s)"
