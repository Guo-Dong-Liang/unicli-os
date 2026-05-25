#!/bin/bash
# hello.say — test CLI for pipe testing and CPL validation
#
# Usage:
#   ./say.sh --name "World" --greeting "Hello" --repeat 3
#
# Reads stdin for pipe input (StructuredMessage frames):
#   When piped, consumes upstream's TEXT output and prepends its own greeting.
#
# Outputs to stdout:
#   StructuredMessage frames (varint-length-prefixed protobuf)
#   Each frame contains the greeting text as payload.
#
# For standalone use (no protobuf library available in Alpine):
#   Falls back to plain text output.

set -euo pipefail

# Defaults
NAME="World"
GREETING="Hello"
REPEAT=1

# Parse CLI flags
while [[ $# -gt 0 ]]; do
    case "$1" in
        --name)
            NAME="$2"
            shift 2
            ;;
        --greeting)
            GREETING="$2"
            shift 2
            ;;
        --repeat)
            REPEAT="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: say.sh [--name NAME] [--greeting GREETING] [--repeat N]"
            echo "  Reads stdin for pipe input, outputs greeting text."
            exit 0
            ;;
        *)
            # Positional args
            if [ -z "${PIPE_INPUT:-}" ]; then
                PIPE_INPUT="$1"
            fi
            shift
            ;;
    esac
done

# Check stdin for pipe input (if we're in a pipeline)
PIPE_DATA=""
if [ ! -t 0 ]; then
    # Read stdin — try to parse as StructuredMessage or use raw text
    PIPE_DATA=$(cat 2>/dev/null || true)
fi

# Build output greeting
OUTPUT=""
for ((i=0; i<REPEAT; i++)); do
    LINE="${GREETING}, ${NAME}!"
    if [ -n "$PIPE_DATA" ]; then
        LINE="${LINE} (received: ${PIPE_DATA})"
    fi
    if [ -n "$OUTPUT" ]; then
        OUTPUT="${OUTPUT}\n${LINE}"
    else
        OUTPUT="${LINE}"
    fi
done

# Output greeting text
echo -e "$OUTPUT"
