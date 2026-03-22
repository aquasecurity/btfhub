#!/bin/bash
#
# Reusable helpers for GitHub Actions: safe ref validation, GITHUB_OUTPUT writes,
# job summaries ($GITHUB_STEP_SUMMARY), and GitHub API utilities.
# Source this file in a step, then call the helpers as needed.
#
# Usage (in a workflow step):
#   source .github/scripts/ci-helpers.sh
#   validate_ref "field_name" "${SOME_REF}"
#   validate_relative_workdir "run-on" "${RUN_ON}"
#   validate_numeric "pr" "${SOME_PR}"
#   set_output "output_name" "${VALUE}"
#   { echo "## Section"; echo "line"; } | summary_append
#   commit_subject "owner/repo" "${TOKEN}" "${SHA}"
#

# Validate a ref value: must be non-empty, single-line, alphanumeric with . _ - /
# Arguments:
#   $1 - name of the field (for error messages)
#   $2 - value to validate
validate_ref() {
    local name="$1"
    local value="$2"

    if [[ -z "${value}" ]]; then
        echo "::error::${name} is empty"
        exit 1
    fi

    local clean
    clean=$(printf '%s' "${value}" | tr -d '\n\r')
    if [[ "${clean}" != "${value}" ]]; then
        echo "::error::${name} contains newline characters (possible injection)"
        exit 1
    fi

    if ! printf '%s' "${value}" | grep -qE '^[a-zA-Z0-9._/\-]+$'; then
        echo "::error::${name} contains invalid characters: ${value}"
        exit 1
    fi
}

# Composite-action working-directory / run-on: ref-style chars plus no ".." or absolute paths.
# Arguments:
#   $1 - name of the field (for error messages)
#   $2 - value to validate
validate_relative_workdir() {
    local name="$1"
    local value="$2"

    validate_ref "${name}" "${value}"
    case "${value}" in
        *..*)
            echo "::error::${name} must not contain .. (path traversal)"
            exit 1
            ;;
        /*)
            echo "::error::${name} must be a relative path"
            exit 1
            ;;
    esac
}

# Validate an optional numeric value (e.g. PR number): must be digits only.
# Skips validation if the value is empty.
# Arguments:
#   $1 - name of the field (for error messages)
#   $2 - value to validate
validate_numeric() {
    local name="$1"
    local value="$2"

    if [[ -n "${value}" ]] && ! [[ "${value}" =~ ^[0-9]+$ ]]; then
        echo "::error::${name} must be a number, got: ${value}"
        exit 1
    fi
}

# Write an output safely using the delimiter form to prevent injection
# Arguments:
#   $1 - output name
#   $2 - output value
# Requires: GITHUB_OUTPUT set (e.g. in GitHub Actions)
set_output() {
    local name="$1"
    local value="$2"
    local delimiter

    delimiter=$(od -An -tx1 -N16 /dev/urandom | tr -d ' \n')
    {
        echo "${name}<<${delimiter}"
        echo "${value}"
        echo "${delimiter}"
    } >> "${GITHUB_OUTPUT}"
}

# Append markdown to the job summary (stdin). No-op if GITHUB_STEP_SUMMARY is unset.
# Usage: summary_append <<'EOF'
# ## My section
# EOF
summary_append() {
    [[ -n "${GITHUB_STEP_SUMMARY:-}" ]] || return 0
    cat >> "${GITHUB_STEP_SUMMARY}"
}

# Fetch the first line (subject) of a commit message via the GitHub API.
# Returns an empty string on failure (best-effort).
# Arguments:
#   $1 - repository (owner/repo)
#   $2 - GitHub token
#   $3 - commit SHA or ref
commit_subject() {
    curl -fsS --connect-timeout 5 --max-time 10 \
        -H "Authorization: token ${2}" \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${1}/commits/${3}" 2> /dev/null \
        | jq -r '.commit.message // "" | split("\n") | .[0] // ""' 2> /dev/null \
        || true
}
