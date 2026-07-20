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
max_output_bytes=${BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES:-16777216}
case "$max_output_bytes" in
  ''|*[!0-9]*)
    printf 'error: benchmark output limit must be a positive byte count\n' >&2
    exit 2
    ;;
esac
if [ "$max_output_bytes" -le 0 ]; then
  printf 'error: benchmark output limit must be a positive byte count\n' >&2
  exit 2
fi
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
repo_root=$(pwd -P)

configured_output=${BLUETAPE_PROVIDER_BENCH_OUTPUT_DIR:-docs/research/outputs/issue-560}
case "$configured_output" in
  docs/research/outputs/issue-560 | "$repo_root/docs/research/outputs/issue-560")
    output_dir=$repo_root/docs/research/outputs/issue-560
    ;;
  *)
    printf 'error: output override must name docs/research/outputs/issue-560 in this repository\n' >&2
    exit 2
    ;;
esac
output_rel=docs/research/outputs/issue-560

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
  /*) ;;
  *) private_parent=$repo_root/$private_parent ;;
esac
if ! private_parent=$(cd "$private_parent" 2>/dev/null && pwd -P); then
  private_parent=$(cd /tmp && pwd -P)
fi
case "$private_parent" in
  "$repo_root" | "$repo_root"/*) private_parent=$(cd /tmp && pwd -P) ;;
esac
private_dir=$(mktemp -d "$private_parent/bluetape-provider-benchmark.XXXXXX")
private_dir=$(cd "$private_dir" && pwd -P)
case "$private_dir" in
  "$repo_root" | "$repo_root"/*)
    rm -rf "$private_dir"
    printf 'error: private capture directory resolved inside the repository\n' >&2
    exit 2
    ;;
esac
chmod 700 "$private_dir"
trap 'rm -rf "$private_dir"' EXIT

raw_file=$private_dir/raw.txt
sanitized_file=$private_dir/sanitized.txt
metadata_file=$private_dir/metadata.txt
: >"$raw_file"
: >"$metadata_file"

git_sha=$(git rev-parse HEAD)
capture_stamp=$(date -u '+%Y%m%dT%H%M%SZ')-$$

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
    printf 'max_output_bytes: %s\n' "$max_output_bytes"
  } >>"$destination"
}

run_command() {
  local label=$1
  shift

  append_header "$metadata_file" "$label" "$@"
  append_header "$raw_file" "$label" "$@"
  printf 'output_begin\n' >>"$raw_file"

  local output_start
  output_start=$(wc -c <"$raw_file" | tr -d '[:space:]')
  set +e
  "$@" 2>&1 | head -c "$max_output_bytes" >>"$raw_file"
  local pipe_status=("${PIPESTATUS[@]}")
  set -e
  local status=${pipe_status[0]}
  local output_end
  output_end=$(wc -c <"$raw_file" | tr -d '[:space:]')
  if [ "${pipe_status[1]}" -ne 0 ] || [ "$((output_end - output_start))" -ge "$max_output_bytes" ]; then
    printf '\n[output_truncated_at_%s_bytes]\n' "$max_output_bytes" >>"$raw_file"
    status=125
  fi

  printf 'output_end\n' >>"$raw_file"
  printf 'exit_status: %s\n' "$status" >>"$raw_file"
  printf 'exit_status: %s\n' "$status" >>"$metadata_file"
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
    if (lower ~ /-----begin [^-]*private key-----/) {
      in_private_key = 1
      print "[redacted_output_line]"
      next
    }
    if (in_private_key) {
      if (lower ~ /-----end [^-]*private key-----/) {
        in_private_key = 0
      }
      next
    }
    if (lower ~ /(^|[^[:alnum:]])(password|passwd|token|secret|authorization|credential|access_key|api_key|private_key|client_secret|endpoint|dsn|proxy|registry|container_id|containerid)[[:space:]]*[=:][[:space:]]*[^[:space:]]+/ ||
        lower ~ /"(password|passwd|token|secret|authorization|credential|access_key|api_key|private_key|client_secret|endpoint|dsn|proxy|registry|container_id|containerid)"[[:space:]]*:/ ||
        $0 ~ /[[:alpha:]][[:alnum:]+.-]*:\/\// ||
        $0 ~ /\/Users\/[^\/[:space:]]+/ ||
        $0 ~ /\/home\/[^\/[:space:]]+/ ||
        $0 ~ /\/private\/(var|tmp)\/[^[:space:]]+/ ||
        $0 ~ /\/var\/folders\/[^[:space:]]+/ ||
        $0 ~ /\/tmp\/[^[:space:]]+/ ||
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
    '([[:alpha:]][[:alnum:]+.-]*:\/\/|(^|[^[:alnum:]])(password|passwd|token|secret|authorization|credential|access_key|api_key|private_key|client_secret|endpoint|dsn|proxy|registry|container_id|containerid)[[:space:]]*[=:][[:space:]]*[^[:space:]]+|"(password|passwd|token|secret|authorization|credential|access_key|api_key|private_key|client_secret|endpoint|dsn|proxy|registry|container_id|containerid)"[[:space:]]*:|-----BEGIN [^-]*PRIVATE KEY-----|-----END [^-]*PRIVATE KEY-----|\/Users\/[^\/[:space:]]+|\/home\/[^\/[:space:]]+|\/private\/(var|tmp)\/[^[:space:]]+|\/var\/folders\/[^[:space:]]+|\/tmp\/[^[:space:]]+|(localhost|host\.docker\.internal):[0-9]+|([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+)' \
    "$1"
}

publish_file() {
  local source=$1
  local target=$2
  local artifact_tmp=''

  artifact_tmp=$(mktemp "$output_dir/.${family}.tmp.XXXXXX") || return 1
  if ! cp "$source" "$artifact_tmp"; then
    rm -f "$artifact_tmp"
    return 1
  fi
  if ! chmod 600 "$artifact_tmp"; then
    rm -f "$artifact_tmp"
    return 1
  fi
  if contains_prohibited_content "$artifact_tmp"; then
    rm -f "$artifact_tmp"
    return 1
  fi
  if ! mv -f "$artifact_tmp" "$target"; then
    rm -f "$artifact_tmp"
    return 1
  fi
}

publish_blocked_metadata() {
  local target=$1
  local blocked=$private_dir/blocked.txt

  cp "$metadata_file" "$blocked"
  printf 'redaction_status: blocked\n' >>"$blocked"
  publish_file "$blocked" "$target"
}

if contains_prohibited_content "$sanitized_file"; then
  if [ "$status" -eq 0 ]; then
    status=125
  fi
  if ! publish_blocked_metadata "$output_dir/$family-failed-$capture_stamp.txt"; then
    printf 'error: unable to retain blocked-redaction metadata\n' >&2
  fi
  printf 'error: prohibited content remained after sanitization; stream body discarded\n' >&2
  exit "$status"
fi

if [ "$status" -eq 0 ]; then
  if ! publish_file "$sanitized_file" "$output_dir/$family.txt"; then
    status=125
    if ! publish_blocked_metadata "$output_dir/$family-failed-$capture_stamp.txt"; then
      printf 'error: unable to retain blocked-redaction metadata\n' >&2
    fi
    printf 'error: artifact-local redaction scan failed; stream body discarded\n' >&2
    exit "$status"
  fi
  printf 'captured %s\n' "$output_dir/$family.txt"
  exit 0
fi

if ! publish_file "$sanitized_file" "$output_dir/$family-failed-$capture_stamp.txt"; then
  if ! publish_blocked_metadata "$output_dir/$family-failed-$capture_stamp.txt"; then
    printf 'error: unable to retain blocked-redaction metadata\n' >&2
  fi
  printf 'error: artifact-local redaction scan failed; stream body discarded\n' >&2
fi
printf 'benchmark family %s failed with exit status %s\n' "$family" "$status" >&2
exit "$status"
