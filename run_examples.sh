#!/bin/bash

# run_examples.sh - Script to test the ACP examples with timeout and cleanup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up processes...${NC}"
    pkill -f example-client 2>/dev/null || true
    pkill -f example-agent 2>/dev/null || true
    wait 2>/dev/null || true
}

# Set trap to cleanup on exit
trap cleanup EXIT

echo -e "${GREEN}Building examples...${NC}"
make examples

echo -e "${GREEN}Starting examples test...${NC}"

# Create named pipes for communication
TEMP_DIR=$(mktemp -d)
INPUT_PIPE="$TEMP_DIR/input"
OUTPUT_FILE="$TEMP_DIR/output"

mkfifo "$INPUT_PIPE"

# Start the client/agent in background
./bin/example-client ./bin/example-agent < "$INPUT_PIPE" > "$OUTPUT_FILE" 2>&1 &
CLIENT_PID=$!

# Give it a moment to start
sleep 2

# Send test input
echo -e "${YELLOW}Sending 'hello' to agent...${NC}"
echo "hello" > "$INPUT_PIPE" &
INPUT_PID=$!

# Wait for a bit to see if it gets stuck
sleep 5

# Check if the process is still running
if kill -0 $CLIENT_PID 2>/dev/null; then
    echo -e "${RED}Process still running - potential deadlock detected${NC}"
    
    # Show output so far
    echo -e "${YELLOW}Output so far:${NC}"
    cat "$OUTPUT_FILE" 2>/dev/null || true
    
    # Kill the processes
    kill $CLIENT_PID 2>/dev/null || true
    kill $INPUT_PID 2>/dev/null || true
    
    echo -e "${RED}DEADLOCK CONFIRMED${NC}"
    exit 1
else
    echo -e "${GREEN}Process completed normally${NC}"
    cat "$OUTPUT_FILE"
    exit 0
fi