#!/usr/bin/env bash
# Print the next plan/branch number based on the remote PR count.
# Usage: ./hack/next-plan-number.sh
set -euo pipefail

repo="functions-dev/ocp-console-plugin"
highest=$(gh pr list --repo "$repo" --state all --json number --jq '.[].number' | sort -n | tail -1)
printf '%03d\n' $(( ${highest:-0} + 1 ))
