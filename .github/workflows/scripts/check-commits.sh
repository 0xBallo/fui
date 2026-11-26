#!/usr/bin/env bash
# check-commits.sh validates every commit header in the given `git log` range against the
# Conventional Commits format from AGENTS.md: "<type>(<scope>): <imperative lowercase summary>",
# scope optional, header at most 72 characters, no AI-tool attribution. Merge commits are skipped:
# GitHub checks out a PR as a merge commit, and merging main back into develop is part of the flow.
set -euo pipefail
shopt -s extglob

range="${1:?usage: check-commits.sh <rev-range>}"

types='feat|fix|test|refactor|perf|chore|style|build|ci|docs'
scopes='domain|app|cli|tui|store|import|connector/revolut|connector/widiba|proto|config|skill'
header_re="^(${types})(\\((${scopes})\\))?: [a-z0-9].{0,69}\$"

# Squash commits already on develop whose titles break the format; history is not rewritten, so they
# are skipped to let the release range pass.
known_bad=' 1faebc569f56495df15f0331b13c1ca6deaf5b92 57b3f3fde7e59dc9e32a3421899d8f614fbdad5f '

status=0
while IFS= read -r sha; do
	[ -z "$sha" ] && continue
	[[ "$known_bad" == *" $sha "* ]] && continue
	header=$(git log -1 --format=%s "$sha")
	# GitHub appends " (#N)" to squash-merge titles; the 72-character limit applies to the title we wrote.
	header="${header% (#+([0-9]))}"
	body=$(git log -1 --format=%b "$sha")

	if ! [[ "$header" =~ $header_re ]]; then
		echo "::error::$sha: header does not match '<type>(<scope>): <summary>' (<=72 chars, lowercase summary): $header"
		status=1
	fi
	# Block AI attribution, not file names in a squashed PR description (e.g. GEMINI.md) nor the human
	# Co-authored-by trailer GitHub adds when squash-merging.
	ai='claude|copilot|chatgpt|gemini|openai|anthropic|ai assistant|\[bot\]'
	if printf '%s\n' "$header" | grep -qiE "$ai" ||
		printf '%s\n' "$body" | grep -iE '^co-authored-by:' | grep -qiE "$ai" ||
		printf '%s\n' "$body" | grep -qiE "generated (with|by) .{0,20}($ai)"; then
		echo "::error::$sha: commit message must not attribute the change to an AI tool"
		status=1
	fi
done < <(git log --no-merges --format=%H "$range")

exit $status
