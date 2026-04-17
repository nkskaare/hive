#!/bin/bash
set -e

echo "🐝 Worker initializing..."

# Non-critical steps
set +e

# Install dependencies if needed
if [ -f "package-lock.json" ]; then
    npm install
elif [ -f "pnpm-lock.yaml" ]; then
    pnpm install --offline || pnpm install
fi

set -e

echo "🚀 Worker ready: $TASK_ID"
exec "$@"
