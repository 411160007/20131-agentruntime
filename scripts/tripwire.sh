#!/usr/bin/env bash
# tripwire.sh — negative scan for internal-ledger identifiers and owner
# contact data leaking into product source/docs (red-line self-check).
#
# Scans every tracked+untracked(non-ignored) text file in the repo.
# Any hit = FAIL. --selftest proves EVERY pattern fires on a known-bad
# fixture (positive control per pattern) and stays silent on known-good
# input (negative control).
#
# Exclusion note: this file itself carries the banned strings as pattern
# definitions, so it is excluded from the real scan. That is the standard
# position for a pattern list; new patterns must never be written as plain
# literals elsewhere in the repo.
set -uo pipefail
cd "$(dirname "$0")/.."

PATTERNS=(
  '\bD-[0-9]{3}\b'
  '\bE[0-9]{2,3}\b'
  '\bT[0-9]{2,3}\b'
  '\bR[0-9]\b'
  '\bPLAN\b'
  'NIGHT_TASKS|TASKBOOK|DECISION_LOG|PROJECT_STATUS|LESSONS|EVOLUTION_LOG'
  'ai-company|/home/node/clawd|clawd'
  '411160007'
  'xstn'
  '[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+\.[A-Za-z0-9.-]+'
)

FIRED=0

scan_files() { # stdin: newline-separated file list
  local files hits i pat
  files=$(cat)
  FIRED=0
  for i in "${!PATTERNS[@]}"; do
    pat="${PATTERNS[$i]}"
    hits=''
    if [ -n "$files" ]; then
      hits=$(printf '%s\n' "$files" | xargs -d '\n' grep -lE "$pat" -- 2>/dev/null || true)
    fi
    if [ -n "$hits" ]; then
      echo "TRIPWIRE HIT pattern[$i] [$pat]:"
      printf '%s\n' "$hits" | head -3 | while read -r f; do
        grep -noE "$pat" -- "$f" 2>/dev/null | head -2 | sed 's/^/    /'
      done
      FIRED=$((FIRED + 1))
    fi
  done
}

if [ "${1:-}" = "--selftest" ]; then
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  printf 'refs D-042 E105 T99 R3 see PLAN NIGHT_TASKS owner 411160007 xstn mail someone@example.org under /home/node/clawd\n' > "$tmp/bad.txt"
  printf 'public text only: this runtime observes agents locally.\n' > "$tmp/good.txt"

  scan_files <<<"$tmp/bad.txt"
  if [ "$FIRED" -ne "${#PATTERNS[@]}" ]; then
    echo "SELFTEST FAIL: only $FIRED/${#PATTERNS[@]} patterns fired on known-bad fixture"
    exit 1
  fi
  echo "SELFTEST OK: all ${#PATTERNS[@]} patterns fired on known-bad fixture"
  scan_files <<<"$tmp/good.txt"
  if [ "$FIRED" -ne 0 ]; then
    echo "SELFTEST FAIL: $FIRED pattern(s) fired on clean fixture"
    exit 1
  fi
  echo "SELFTEST OK: clean fixture fired nothing"
  exit 0
fi

files=$(git ls-files --cached --others --exclude-standard | grep -v '^scripts/tripwire\.sh$' || true)
scan_files <<<"$files"
if [ "$FIRED" -ne 0 ]; then
  echo "TRIPWIRE FAIL: $FIRED pattern group(s) hit"
  exit 1
fi
n=$(printf '%s\n' "$files" | sed '/^$/d' | wc -l)
echo "TRIPWIRE CLEAN ($n files scanned)"
