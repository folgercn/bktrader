#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

chmod +x scripts/install_git_hooks.sh scripts/run_changed_scope_checks.sh .githooks/pre-push
chmod +x scripts/find_graphify_python.sh
git config --local core.hooksPath .githooks

echo "Git hooks installed."
echo "core.hooksPath=$(git config --local core.hooksPath)"
