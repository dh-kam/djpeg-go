#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/bump-up.sh --semver X.Y.Z --upstream-version VERSION [--period YYYYMM]

Prints the next tag in the form:
  vSEMVER-YYYYMM.seq-upstreamversion
USAGE
}

semver=""
upstream_version=""
period="$(date -u +%Y%m)"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --semver)
      semver="${2:-}"
      shift 2
      ;;
    --upstream-version)
      upstream_version="${2:-}"
      shift 2
      ;;
    --period)
      period="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if ! [[ "$semver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "--semver must be MAJOR.MINOR.PATCH, got: ${semver:-<empty>}" >&2
  exit 2
fi
if ! [[ "$period" =~ ^[0-9]{6}$ ]]; then
  echo "--period must be YYYYMM, got: $period" >&2
  exit 2
fi
if ! [[ "$upstream_version" =~ ^[0-9A-Za-z._-]+$ ]]; then
  echo "--upstream-version must contain only letters, numbers, dot, underscore, or dash" >&2
  exit 2
fi

pattern="v${semver}-${period}.*-${upstream_version}"
max_seq=0

while IFS= read -r tag; do
  [ -n "$tag" ] || continue
  rest="${tag#v${semver}-${period}.}"
  seq="${rest%-${upstream_version}}"
  if [[ "$seq" =~ ^[0-9]+$ ]] && [ "$seq" -gt "$max_seq" ]; then
    max_seq="$seq"
  fi
done < <(git tag --list "$pattern")

next_seq=$((max_seq + 1))
printf 'v%s-%s.%d-%s\n' "$semver" "$period" "$next_seq" "$upstream_version"
