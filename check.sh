#!/usr/bin/env bash
# Runs the checkpoint tests. See CHECKPOINTS.md.
#
#   ./check.sh list        list every checkpoint
#   ./check.sh 1.1         run one checkpoint
#   ./check.sh 4           run every checkpoint in milestone 4
#   ./check.sh all         run everything (except the 2.1 race demo)
#
# Extra arguments go to `go test`, e.g. ./check.sh 4.5 -v
set -euo pipefail
cd "$(dirname "$0")"

usage() { sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }
[[ $# -ge 1 ]] || usage
arg=$1; shift

if [[ $arg == list ]]; then
  grep -rh --include='checkpoint_*_test.go' -E '^// Checkpoint [0-9]+\.[0-9]+:' . \
    | sed 's#^// Checkpoint ##' | sort -t. -k1,1n -k2,2n | uniq
  exit 0
fi

case $arg in
  all) tag=all ;;
  [0-9]) tag=m$arg ;;
  [0-9].[0-9]) tag=m${arg/./_} ;;
  *) usage ;;
esac

dirs=$(grep -rlE --include='checkpoint_*_test.go' "^//go:build (.* )?$tag( .*)?\$" . \
  | xargs -n1 dirname | sort -u)
if [[ -z $dirs ]]; then
  echo "No checkpoint called '$arg'. Try ./check.sh list"
  exit 1
fi

# Live Alpaca checks (5.3) read your keys from .env if it exists.
if [[ -f .env ]]; then set -a; source .env; set +a; fi

if [[ $tag == m2_1 ]]; then
  echo "Note: checkpoint 2.1 is SUPPOSED to fail with 'WARNING: DATA RACE'. Read the report."
  echo
fi

# shellcheck disable=SC2086
if go test -count=1 -race -timeout 120s -tags "$tag" $dirs "$@"; then
  echo
  echo "✅ checkpoint $arg passed"
else
  echo
  echo "❌ checkpoint $arg has failures (see above)"
  exit 1
fi
