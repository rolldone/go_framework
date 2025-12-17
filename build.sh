#!/usr/bin/env bash

# Compute repository root (the directory containing this script). This makes
# relative paths robust when the script is invoked from another CWD or inside
# a container where the working directory differs from the repository root.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# If a go.mod exists, tidy dependencies so tooling (swag, build) can run cleanly.
if [ -f go.mod ]; then
	echo "Found go.mod — running 'go mod tidy' to ensure deps are present"
	if ! go mod tidy; then
		echo "go mod tidy failed; continuing without failing the build script"
	fi
else
	echo "go.mod not found — skipping 'go mod tidy'"
fi

go build -o ./bin/console ./cmd/console

echo "Build console completed. Binary is located at ./bin/console"

# Regenerate swagger docs if swag CLI is available (non-fatal).
echo "Attempting to regenerate swagger docs (if swag CLI is installed)"
if command -v swag >/dev/null 2>&1; then
    echo "Found swag CLI - regenerating docs..."
	# Ensure docs output directory exists and run swag to output into ./docs/swagger
	mkdir -p ./docs/swagger
	# Generate into ./docs/swagger so the server can serve ./docs/swagger/swagger.json
	swag init -g cmd/server/main.go -o ./docs/swagger || echo "swag init returned non-zero status (continuing)"
else
    echo "swag CLI not found in PATH; skipping docs generation"
fi

go build -o ./bin/server ./cmd/server

echo "Build server completed. Binary is located at ./bin/server"