#!/usr/bin/env bash

set -eu

usage() {
  printf 'usage: %s <family>\n' "${0##*/}" >&2
  printf '%s\n' 'families: leader-local leader-containers leader-probes ratelimit-local ratelimit-containers cache-local cache-redis graphio graphdb' >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

family=$1
case "$family" in
  leader-local | leader-containers | leader-probes | ratelimit-local | ratelimit-containers | cache-local | cache-redis | graphio | graphdb) ;;
  *)
    usage
    exit 2
    ;;
esac

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  printf 'error: benchmark capture must run inside a Git worktree\n' >&2
  exit 2
}
cd "$repo_root"

configured_output=${BLUETAPE_PROVIDER_BENCH_OUTPUT_DIR:-docs/research/outputs/issue-560}
case "$configured_output" in
  /*) output_dir=$configured_output ;;
  *) output_dir=$repo_root/$configured_output ;;
esac
case "$output_dir" in
  "$repo_root" | *'/../'* | *'/..')
    printf 'error: output directory must be a child of the repository\n' >&2
    exit 2
    ;;
  "$repo_root"/*) ;;
  *)
    printf 'error: output directory must be a child of the repository\n' >&2
    exit 2
    ;;
esac
output_rel=${output_dir#"$repo_root"/}

dirty=$(git status --porcelain=v1 --untracked-files=all -- . \
  ":(exclude)$output_rel" ":(exclude)$output_rel/**")
if [ -n "$dirty" ]; then
  printf 'error: benchmark capture requires a clean worktree outside %s\n' "$output_rel" >&2
  printf '%s\n' "$dirty" >&2
  exit 2
fi

mkdir -p "$output_dir"
resolved_output=$(cd "$output_dir" && pwd -P)
case "$resolved_output" in
  "$repo_root"/*) output_dir=$resolved_output ;;
  *)
    printf 'error: resolved output directory escapes the repository\n' >&2
    exit 2
    ;;
esac

umask 077
private_parent=${TMPDIR:-/tmp}
case "$private_parent" in
  "$repo_root" | "$repo_root"/*) private_parent=/tmp ;;
esac
private_dir=$(mktemp -d "$private_parent/bluetape-provider-benchmark.XXXXXX")
chmod 700 "$private_dir"
trap 'rm -rf "$private_dir"' EXIT HUP INT TERM

raw_file=$private_dir/raw.txt
sanitized_file=$private_dir/sanitized.txt
metadata_file=$private_dir/metadata.txt
: >"$raw_file"
: >"$metadata_file"

git_sha=$(git rev-parse HEAD)
capture_stamp=$(date -u '+%Y%m%dT%H%M%SZ')

display_command() {
  local token
  printf 'command:'
  for token in "$@"; do
    printf ' %q' "$token"
  done
  printf '\n'
}

append_header() {
  local destination=$1
  local label=$2
  shift 2

  {
    printf 'command_label: %s\n' "$label"
    display_command "$@"
    printf 'timestamp_utc: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf 'git_sha: %s\n' "$git_sha"
    printf 'pre_run_clean: true\n'
  } >>"$destination"
}

run_command() {
  local label=$1
  shift

  append_header "$metadata_file" "$label" "$@"
  append_header "$raw_file" "$label" "$@"
  printf 'output_begin\n' >>"$raw_file"

  set +e
  "$@" >>"$raw_file" 2>&1
  local status=$?
  set -e

  printf 'output_end\n' >>"$raw_file"
  printf 'exit_status: %s\n\n' "$status" >>"$raw_file"
  printf 'exit_status: %s\n\n' "$status" >>"$metadata_file"
  return "$status"
}

status=0
case "$family" in
  leader-local)
    set -- go test -timeout=10m -run '^$' -bench '^BenchmarkProviderLeaderLocal$' -benchmem -count=5 ./leader
    run_command leader-local "$@" || status=$?
    ;;
  leader-containers)
    set -- env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/(CampaignUncontended|ResignOwned|CampaignContention|LeaderLookup)$' -benchtime=100x -count=3 -benchmem ./leader
    if run_command ordinary "$@"; then
      set -- env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=10m -p 1 -run '^$' -bench '^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/ExpiryTakeover$' -benchtime=1x -count=3 -benchmem ./leader
      run_command expiry "$@" || status=$?
    else
      status=$?
    fi
    ;;
  leader-probes)
    set -- env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=15m -p 1 -run '^TestProviderLeaderBenchmarkProbes$' ./leader
    run_command leader-probes "$@" || status=$?
    ;;
  ratelimit-local)
    set -- go test -timeout=10m -run '^$' -bench '^BenchmarkProviderRateLimitLocal$' -benchmem -count=5 ./ratelimit
    run_command ratelimit-local "$@" || status=$?
    ;;
  ratelimit-containers)
    set -- env BLUETAPE_RATELIMIT_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderRateLimitContainers$' -benchtime=100x -count=3 -benchmem ./ratelimit
    run_command ratelimit-containers "$@" || status=$?
    ;;
  cache-local)
    set -- go test -timeout=10m -run '^$' -bench '^BenchmarkProviderCacheLocal$' -benchmem -count=5 ./cache
    run_command cache-local "$@" || status=$?
    ;;
  cache-redis)
    set -- env BLUETAPE_CACHE_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderCacheRedis$' -benchtime=100x -count=3 -benchmem ./cache
    run_command cache-redis "$@" || status=$?
    ;;
  graphio)
    set -- go test -timeout=10m -run '^$' -bench '^BenchmarkGraphIOFormats$' -benchmem -count=5 ./graph/graphio
    run_command graphio "$@" || status=$?
    ;;
  graphdb)
    set -- env BLUETAPE_GRAPH_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkGraphProviderTraversalContainers$' -benchtime=100x -count=3 -benchmem ./graph
    run_command graphdb "$@" || status=$?
    ;;
esac

awk '
  {
    lower = tolower($0)
    if (lower ~ /(^|[^[:alnum:]_])(password|passwd|token|secret|authorization)[[:space:]]*[=:][[:space:]]*[^[:space:]]+/ ||
        $0 ~ /:\/\/[^\/@[:space:]]+:[^\/@[:space:]]+@/ ||
        $0 ~ /\/Users\/[^\/[:space:]]+/ ||
        $0 ~ /\/home\/[^\/[:space:]]+/ ||
        lower ~ /(localhost|host\.docker\.internal):[0-9]+/ ||
        $0 ~ /([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+/) {
      print "[redacted_output_line]"
      next
    }
    print
  }
' "$raw_file" >"$sanitized_file"

contains_prohibited_content() {
  LC_ALL=C grep -Eiq \
    '(:\/\/[^\/@[:space:]]+:[^\/@[:space:]]+@|(^|[^[:alnum:]_])(password|passwd|token|secret|authorization)[[:space:]]*[=:][[:space:]]*[^[:space:]]+|\/Users\/[^\/[:space:]]+|\/home\/[^\/[:space:]]+|(localhost|host\.docker\.internal):[0-9]+|([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+)' \
    "$1"
}

publish_file() {
  local source=$1
  local target=$2
  local artifact_tmp

  artifact_tmp=$(mktemp "$output_dir/.${family}.tmp.XXXXXX")
  cp "$source" "$artifact_tmp"
  chmod 600 "$artifact_tmp"
  mv -f "$artifact_tmp" "$target"
}

if contains_prohibited_content "$sanitized_file"; then
  blocked=$private_dir/blocked.txt
  cp "$metadata_file" "$blocked"
  printf 'redaction_status: blocked\n' >>"$blocked"
  if [ "$status" -eq 0 ]; then
    status=125
  fi
  publish_file "$blocked" "$output_dir/$family-failed-$capture_stamp.txt"
  printf 'error: prohibited content remained after sanitization; stream body discarded\n' >&2
  exit "$status"
fi

if [ "$status" -eq 0 ]; then
  publish_file "$sanitized_file" "$output_dir/$family.txt"
  printf 'captured %s\n' "$output_dir/$family.txt"
  exit 0
fi

publish_file "$sanitized_file" "$output_dir/$family-failed-$capture_stamp.txt"
printf 'benchmark family %s failed with exit status %s\n' "$family" "$status" >&2
exit "$status"
