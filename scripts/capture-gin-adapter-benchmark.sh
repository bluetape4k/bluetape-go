#!/usr/bin/env bash

set -euo pipefail

usage() {
  printf 'usage: %s <count> <cpu-list>\n' "${0##*/}" >&2
  printf '%s\n' 'example: bench-web-gin 5 1,2,4' >&2
}

if [ "$#" -ne 2 ]; then
  usage
  exit 2
fi

count=$1
cpu_list=$2
case "$count" in
  ''|*[!0-9]*)
    printf 'error: benchmark count must be a positive integer\n' >&2
    exit 2
    ;;
esac
if [ "$count" -le 0 ]; then
  printf 'error: benchmark count must be a positive integer\n' >&2
  exit 2
fi

IFS=',' read -r -a cpu_values <<<"$cpu_list"
if [ "${#cpu_values[@]}" -eq 0 ]; then
  printf 'error: CPU list must contain at least one positive integer\n' >&2
  exit 2
fi
for cpu in "${cpu_values[@]}"; do
  case "$cpu" in
    ''|*[!0-9]*)
      printf 'error: CPU list must contain only positive integers: %s\n' "$cpu_list" >&2
      exit 2
      ;;
  esac
  if [ "$cpu" -le 0 ]; then
    printf 'error: CPU list must contain only positive integers: %s\n' "$cpu_list" >&2
    exit 2
  fi
done

if [[ ",${cpu_list}," != *,1,* ]]; then
  printf 'error: CPU list must include 1 for the canonical CPU 1 chart summary\n' >&2
  exit 2
fi

default_max_output_bytes=10485760
max_output_bytes=${BLUETAPE_GIN_BENCH_MAX_OUTPUT_BYTES:-$default_max_output_bytes}
case "$max_output_bytes" in
  ''|*[!0-9]*)
    printf 'error: benchmark output limit must be a positive byte count\n' >&2
    exit 2
    ;;
esac
if [ "$max_output_bytes" -le 0 ] || [ "$max_output_bytes" -gt "$default_max_output_bytes" ]; then
  printf 'error: benchmark output limit must be between 1 and %s bytes\n' "$default_max_output_bytes" >&2
  exit 2
fi

default_chart_timeout_seconds=60
chart_timeout_seconds=${BLUETAPE_GIN_BENCH_CHART_TIMEOUT_SECONDS:-$default_chart_timeout_seconds}
case "$chart_timeout_seconds" in
  ''|*[!0-9]*)
    printf 'error: chart timeout must be a positive second count\n' >&2
    exit 2
    ;;
esac
if [ "$chart_timeout_seconds" -le 0 ] || [ "$chart_timeout_seconds" -gt 600 ]; then
  printf 'error: chart timeout must be between 1 and 600 seconds\n' >&2
  exit 2
fi
chart_max_output_bytes=${BLUETAPE_GIN_BENCH_CHART_MAX_OUTPUT_BYTES:-$max_output_bytes}
case "$chart_max_output_bytes" in
  ''|*[!0-9]*)
    printf 'error: chart output limit must be a positive byte count\n' >&2
    exit 2
    ;;
esac
if [ "$chart_max_output_bytes" -le 0 ] || [ "$chart_max_output_bytes" -gt "$default_max_output_bytes" ]; then
  printf 'error: chart output limit must be between 1 and %s bytes\n' "$default_max_output_bytes" >&2
  exit 2
fi

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  printf 'error: benchmark capture must run inside a Git worktree\n' >&2
  exit 2
}
cd "$repo_root"
repo_root=$(pwd -P)

configured_output=${BLUETAPE_GIN_BENCH_OUTPUT_DIR:-docs/research/outputs/issue-543}
case "$configured_output" in
  docs/research/outputs/issue-543 | "$repo_root/docs/research/outputs/issue-543")
    output_dir=$repo_root/docs/research/outputs/issue-543
    ;;
  *)
    printf 'error: output override must name docs/research/outputs/issue-543 in this repository\n' >&2
    exit 2
    ;;
esac
output_rel=docs/research/outputs/issue-543
chart_dir=$repo_root/docs/images/readme-charts

mkdir -p "$output_dir" "$chart_dir"
resolved_output=$(cd "$output_dir" && pwd -P)
resolved_chart=$(cd "$chart_dir" && pwd -P)
case "$resolved_output" in
  "$repo_root"/*) output_dir=$resolved_output ;;
  *)
    printf 'error: resolved output directory escapes the repository\n' >&2
    exit 2
    ;;
esac
case "$resolved_chart" in
  "$repo_root"/*) chart_dir=$resolved_chart ;;
  *)
    printf 'error: resolved chart directory escapes the repository\n' >&2
    exit 2
    ;;
esac

# canonical evidence 파일은 capture가 자체 갱신하므로 source cleanliness 판정에서 제외한다.
dirty=$(git status --porcelain=v1 --untracked-files=all -- . \
  ":(exclude)$output_rel" ":(exclude)$output_rel/**" \
  ":(exclude)docs/images/readme-charts/gin-adapter-benchmark-summary.png" \
  ":(exclude)docs/images/readme-charts/gin-adapter-benchmark-summary.svg" \
  ":(exclude)docs/images/readme-charts/gin-adapter-benchmark-summary.vl.json" \
  ":(exclude)docs/research/2026-08-16-issue-543-gin-adapter-benchmark.md")
if [ -n "$dirty" ]; then
  dirty_tree=true
  capture_eligibility=N/A
  no_regression=N/A
else
  dirty_tree=false
  capture_eligibility=eligible
  no_regression=N/A
fi

umask 077
private_parent=${TMPDIR:-/tmp}
case "$private_parent" in
  /*) ;;
  *) private_parent=/tmp ;;
esac
if ! private_parent=$(cd "$private_parent" 2>/dev/null && pwd -P); then
  private_parent=/tmp
fi
private_dir=$(mktemp -d "$private_parent/bluetape-gin-adapter-benchmark.XXXXXX")
private_dir=$(cd "$private_dir" && pwd -P)
case "$private_dir" in
  "$repo_root"|"$repo_root"/*)
    rm -rf "$private_dir"
    printf 'error: private capture directory resolved inside the repository\n' >&2
    exit 2
    ;;
esac
chmod 700 "$private_dir"
failure_written=false
trap 'handle_exit "$?"' EXIT

raw_file=$private_dir/bench-output.txt
sanitized_file=$private_dir/bench-output-sanitized.txt
metadata_file=$private_dir/bench-environment.txt
command_output_file=$private_dir/command-output.txt
results_file=$private_dir/bench-results.json
chart_output_dir=$private_dir/chart
chart_generation_log=$private_dir/chart-generation.log
chart_generation_sanitized_file=$private_dir/chart-generation-sanitized.log
failure_file=$private_dir/failure.txt
capture_stamp=$(date -u '+%Y%m%dT%H%M%SZ')-$$
git_sha=$(git rev-parse HEAD)
timestamp_utc=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
benchmark_pid=''
chart_pid=''
chart_failure_reason=not_run
chart_exit_status=not_run

go_version=$(go version 2>/dev/null || printf 'unknown')
goos=$(go env GOOS 2>/dev/null || printf 'unknown')
goarch=$(go env GOARCH 2>/dev/null || printf 'unknown')
logical_cpus=$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.logicalcpu 2>/dev/null || printf 'unknown')
gin_version=$(go list -m -f '{{.Version}}' github.com/gin-gonic/gin 2>/dev/null || printf 'unknown')
fixture_identity=gin-v1.12.0-parser-only-local

benchmark_command=(go test -timeout=10m -run '^$' -bench '^BenchmarkGinAdapter' -benchmem -count="$count" -cpu="$cpu_list" ./web/gin)

display_command() {
  local token
  printf 'command:'
  for token in "$@"; do
    printf ' %q' "$token"
  done
  printf '\n'
}

write_metadata() {
  local command_string=''
  local token
  for token in "${benchmark_command[@]}"; do
    if [ -n "$command_string" ]; then
      command_string="$command_string "
    fi
    command_string="$command_string$(printf '%q' "$token")"
  done
  {
    printf 'timestamp_utc: %s\n' "$timestamp_utc"
    printf 'git_sha: %s\n' "$git_sha"
    printf 'dirty_tree: %s\n' "$dirty_tree"
    printf 'capture_eligibility: %s\n' "$capture_eligibility"
    printf 'no_regression: %s\n' "$no_regression"
    printf 'gin_version: %s\n' "$gin_version"
    printf 'fixture_identity: %s\n' "$fixture_identity"
    printf 'go_version: %s\n' "$go_version"
    printf 'goos: %s\n' "$goos"
    printf 'goarch: %s\n' "$goarch"
    printf 'cpu: %s\n' "$cpu_list"
    printf 'logical_cpus: %s\n' "$logical_cpus"
    printf 'benchmark_count: %s\n' "$count"
    printf 'max_output_bytes: %s\n' "$max_output_bytes"
    printf 'chart_timeout_seconds: %s\n' "$chart_timeout_seconds"
    printf 'chart_max_output_bytes: %s\n' "$chart_max_output_bytes"
    printf 'command: %s\n' "$command_string"
  } >"$metadata_file"
}

write_metadata

run_benchmark() {
  : >"$command_output_file"
  local command_status=0
  local output_bytes=0
  BLUETAPE_GIN_BENCH_CAPTURE_PID=$$ "${benchmark_command[@]}" >"$command_output_file" 2>&1 &
  benchmark_pid=$!
  while kill -0 "$benchmark_pid" 2>/dev/null; do
    output_bytes=$(wc -c <"$command_output_file" | tr -d '[:space:]')
    if [ "$output_bytes" -gt "$max_output_bytes" ]; then
      kill -TERM "$benchmark_pid" 2>/dev/null || true
      wait "$benchmark_pid" 2>/dev/null || true
      printf '\n[output_truncated_at_%s_bytes]\n' "$max_output_bytes" >>"$command_output_file"
      command_status=125
      break
    fi
    sleep 0.05
  done
  if [ "$command_status" -eq 0 ]; then
    if wait "$benchmark_pid"; then
      command_status=0
    else
      command_status=$?
    fi
  fi
  benchmark_pid=''
  output_bytes=$(wc -c <"$command_output_file" | tr -d '[:space:]')
  if [ "$output_bytes" -gt "$max_output_bytes" ] && [ "$command_status" -eq 0 ]; then
    printf '\n[output_truncated_at_%s_bytes]\n' "$max_output_bytes" >>"$command_output_file"
    command_status=125
  fi

  {
    cat "$metadata_file"
    printf 'output_begin\n'
    cat "$command_output_file"
    printf '\noutput_end\n'
    printf 'exit_status: %s\n' "$command_status"
  } >"$raw_file"
  return "$command_status"
}

sanitize_file() {
  local input=$1
  local output=$2
  LC_ALL=C awk '
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
      if (lower ~ /(^|[^[:alnum:]])(password|passwd|token|secret|authorization|credential|access[-_]?key|access[-_]?token|api[-_]?key|private[-_]?key|client[-_]?secret|endpoint|dsn|proxy|registry|container[-_]?id)[[:space:]]*[=:][[:space:]]*[^[:space:]]+/ ||
          lower ~ /"(password|passwd|token|secret|authorization|credential|access[-_]?key|access[-_]?token|api[-_]?key|private[-_]?key|client[-_]?secret|endpoint|dsn|proxy|registry|container[-_]?id)"[[:space:]]*:/ ||
          $0 ~ /[[:alpha:]][[:alnum:]+.-]*:\/\// ||
          $0 ~ /\/Users\/[^\/[:space:]]+/ ||
          $0 ~ /\/home\/[^\/[:space:]]+/ ||
          $0 ~ /\/private\/(var|tmp)\/[^[:space:]]+/ ||
          $0 ~ /\/var\/folders\/[^[:space:]]+/ ||
          $0 ~ /\/tmp\/[^[:space:]]+/ ||
          lower ~ /(localhost|host\.docker\.internal):[0-9]+/ ||
          lower ~ /(^|[[:space:]])panic:[[:space:]]+/ ||
          lower ~ /(^|[[:space:]])bearer[[:space:]]+[[:alnum:]_.-]+/ ||
          $0 ~ /(^|[[:space:]])eyJ[[:alnum:]_-]+\.[[:alnum:]_-]+\.[[:alnum:]_-]+($|[[:space:]])/ ||
          $0 ~ /([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+/) {
        print "[redacted_output_line]"
        next
      }
      print
    }
  ' "$input" >"$output"
}

sanitize_output() {
  sanitize_file "$raw_file" "$sanitized_file"
}

sanitize_chart_output() {
  if [ -f "$chart_generation_log" ]; then
    sanitize_file "$chart_generation_log" "$chart_generation_sanitized_file"
  else
    : >"$chart_generation_sanitized_file"
  fi
  if contains_prohibited_content "$chart_generation_sanitized_file"; then
    redaction_status=blocked
  fi
}

contains_prohibited_content() {
  LC_ALL=C grep -Eiq \
    '([[:alpha:]][[:alnum:]+.-]*:\/\/|(^|[^[:alnum:]])(password|passwd|token|secret|authorization|credential|access[-_]?key|access[-_]?token|api[-_]?key|private[-_]?key|client[-_]?secret|endpoint|dsn|proxy|registry|container[-_]?id)[[:space:]]*[=:][[:space:]]*[^[:space:]]+|"(password|passwd|token|secret|authorization|credential|access[-_]?key|access[-_]?token|api[-_]?key|private[-_]?key|client[-_]?secret|endpoint|dsn|proxy|registry|container[-_]?id)"[[:space:]]*:|-----BEGIN [^-]*PRIVATE KEY-----|-----END [^-]*PRIVATE KEY-----|\/Users\/[^\/[:space:]]+|\/home\/[^\/[:space:]]+|\/private\/(var|tmp)\/[^[:space:]]+|\/var\/folders\/[^[:space:]]+|\/tmp\/[^[:space:]]+|(localhost|host\.docker\.internal):[0-9]+|(^|[[:space:]])panic:[[:space:]]+|(^|[[:space:]])bearer[[:space:]]+[[:alnum:]_.-]+|(^|[[:space:]])eyJ[[:alnum:]_-]+\.[[:alnum:]_-]+\.[[:alnum:]_-]+($|[[:space:]])|([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+)' \
    "$1"
}

terminate_chart_process() {
  local child
  if [ -z "${chart_pid:-}" ]; then
    return 0
  fi
  for child in $(pgrep -P "$chart_pid" 2>/dev/null || true); do
    kill -TERM "$child" 2>/dev/null || true
  done
  kill -TERM "$chart_pid" 2>/dev/null || true
  wait "$chart_pid" 2>/dev/null || true
  for child in $(pgrep -P "$chart_pid" 2>/dev/null || true); do
    kill -KILL "$child" 2>/dev/null || true
  done
  kill -KILL "$chart_pid" 2>/dev/null || true
  chart_pid=''
}

truncate_chart_log() {
  local temporary=$private_dir/chart-generation-truncated.log
  if [ ! -f "$chart_generation_log" ]; then
    : >"$chart_generation_log"
    return 0
  fi
  if ! head -c "$chart_max_output_bytes" "$chart_generation_log" >"$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  mv -f "$temporary" "$chart_generation_log"
}

run_chart_generation() {
  : >"$chart_generation_log"
  chart_failure_reason=not_run
  chart_exit_status=not_run
  BLUETAPE_GIN_BENCH_CHART_DIR="$chart_output_dir" node \
    "$repo_root/docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs" \
    "$results_file" >"$chart_generation_log" 2>&1 &
  chart_pid=$!

  local chart_elapsed_ms=0
  local output_bytes=0
  local command_status=0
  while kill -0 "$chart_pid" 2>/dev/null; do
    output_bytes=$(wc -c <"$chart_generation_log" | tr -d '[:space:]')
    if [ "$output_bytes" -gt "$chart_max_output_bytes" ]; then
      chart_failure_reason=output_limit
      terminate_chart_process
      truncate_chart_log
      printf '\n[chart_output_truncated_at_%s_bytes]\n' "$chart_max_output_bytes" >>"$chart_generation_log"
      chart_exit_status=125
      return 125
    fi
    if [ "$chart_elapsed_ms" -ge "$((chart_timeout_seconds * 1000))" ]; then
      chart_failure_reason=timeout
      terminate_chart_process
      printf '\n[chart_timeout_after_%s_seconds]\n' "$chart_timeout_seconds" >>"$chart_generation_log"
      chart_exit_status=124
      return 124
    fi
    sleep 0.05
    chart_elapsed_ms=$((chart_elapsed_ms + 50))
  done

  if wait "$chart_pid"; then
    command_status=0
  else
    command_status=$?
  fi
  chart_pid=''
  output_bytes=$(wc -c <"$chart_generation_log" | tr -d '[:space:]')
  if [ "$output_bytes" -gt "$chart_max_output_bytes" ] && [ "$command_status" -eq 0 ]; then
    chart_failure_reason=output_limit
    truncate_chart_log
    printf '\n[chart_output_truncated_at_%s_bytes]\n' "$chart_max_output_bytes" >>"$chart_generation_log"
    command_status=125
  fi
  chart_exit_status=$command_status
  if [ "$command_status" -eq 0 ]; then
    chart_failure_reason=none
  elif [ "$chart_failure_reason" != "output_limit" ] && [ "$command_status" -ge 128 ]; then
    chart_failure_reason=signal
  elif [ "$chart_failure_reason" != "output_limit" ]; then
    chart_failure_reason='exit'
  fi
  return "$command_status"
}

write_failure_metadata() {
  local phase=$1
  local failure_status=$2
  local redaction_status=$3
  {
    printf 'capture_status: failed\n'
    printf 'failure_phase: %s\n' "$phase"
    printf 'failure_exit_status: %s\n' "$failure_status"
    printf 'chart_failure_reason: %s\n' "$chart_failure_reason"
    printf 'chart_exit_status: %s\n' "$chart_exit_status"
    printf 'redaction_status: %s\n' "$redaction_status"
    cat "$metadata_file"
    if [ -f "$sanitized_file" ] && [ "$redaction_status" = "passed" ]; then
      printf 'failure_output_begin\n'
      cat "$sanitized_file"
      printf 'failure_output_end\n'
    fi
    if [ -f "$chart_generation_sanitized_file" ] && [ "$redaction_status" = "passed" ]; then
      printf 'chart_stderr_begin\n'
      cat "$chart_generation_sanitized_file"
      printf 'chart_stderr_end\n'
    fi
  } >"$failure_file"
}

publish_single() {
  local source=$1
  local target=$2
  local temporary
  temporary=$(mktemp "$(dirname "$target")/.$(basename "$target").tmp.XXXXXX") || return 1
  if ! cp "$source" "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  if ! chmod 600 "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  if contains_prohibited_content "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  if ! mv -f "$temporary" "$target"; then
    rm -f "$temporary"
    return 1
  fi
}

publish_failure() {
  local phase=$1
  local failure_status=$2
  local redaction_status=$3
  local failed_target=$output_dir/bench-failed-$capture_stamp.txt
  failure_written=true
  write_failure_metadata "$phase" "$failure_status" "$redaction_status"
  if ! publish_single "$failure_file" "$failed_target"; then
    printf 'error: unable to retain benchmark failure metadata at %s\n' "$failed_target" >&2
  fi
}

handle_exit() {
  local exit_status=$1
  trap - EXIT INT TERM HUP
  if [ -n "${chart_pid:-}" ]; then
    terminate_chart_process || true
  fi
  if { [ "$exit_status" -eq 129 ] || [ "$exit_status" -eq 130 ] || [ "$exit_status" -eq 143 ]; } && \
    [ "${failure_written:-false}" = false ]; then
    if [ "${publication_started:-false}" = true ]; then
      restore_publication || true
      publication_started=false
    fi
    publish_failure "${phase:-unknown}" "$exit_status" "${redaction_status:-not_run}" || true
  fi
  rm -rf "$private_dir"
  return "$exit_status"
}

declare -a staged_files=()
declare -a publish_targets=()
declare -a publish_backups=()
declare -a publish_had_previous=()
stage_file() {
  local source=$1
  local target=$2
  local scan=${3:-true}
  local temporary
  temporary=$(mktemp "$(dirname "$target")/.$(basename "$target").tmp.XXXXXX") || return 1
  if ! cp "$source" "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  if ! chmod 600 "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  if [ "$scan" = true ] && contains_prohibited_content "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  staged_files+=("$temporary$(printf '\t')$target")
}

cleanup_staged() {
  local pair temporary
  for pair in "${staged_files[@]}"; do
    temporary=${pair%%$'\t'*}
    rm -f "$temporary"
  done
}

prepare_publication_backup() {
  local target=$1
  local index=${#publish_targets[@]}
  local backup=$private_dir/backup-$index
  publish_targets+=("$target")
  if [ -e "$target" ]; then
    if ! cp -p "$target" "$backup"; then
      return 1
    fi
    publish_backups+=("$backup")
    publish_had_previous+=(true)
  else
    publish_backups+=("")
    publish_had_previous+=(false)
  fi
}

restore_publication() {
  local index target backup
  local restore_status=0
  for ((index = 0; index < ${#publish_targets[@]}; index += 1)); do
    target=${publish_targets[$index]}
    backup=${publish_backups[$index]}
    if [ "${publish_had_previous[$index]}" = true ]; then
      if ! cp -p "$backup" "$target"; then
        restore_status=1
      fi
    elif ! rm -f "$target"; then
      restore_status=1
    fi
  done
  return "$restore_status"
}

handle_signal() {
  local signal=$1
  local signal_status=143
  case "$signal" in
    INT) signal_status=130 ;;
    HUP) signal_status=129 ;;
  esac
  trap - INT TERM HUP
  status=$signal_status
  if [ -n "${benchmark_pid:-}" ]; then
    kill -TERM "$benchmark_pid" 2>/dev/null || true
  fi
  if [ -n "${chart_pid:-}" ]; then
    terminate_chart_process || true
    chart_failure_reason=signal
    chart_exit_status=$signal_status
    sanitize_chart_output || true
  fi
  if [ "${publication_started:-false}" = true ]; then
    restore_publication || true
    publication_started=false
  fi
  publish_failure "${phase:-unknown}" "$status" "${redaction_status:-not_run}" || true
  printf 'error: Gin adapter benchmark interrupted by SIG%s\n' "$signal" >&2
  exit "$status"
}

phase=setup
status=0
redaction_status=not_run
publication_started=false
trap 'handle_signal INT' INT
trap 'handle_signal TERM' TERM
trap 'handle_signal HUP' HUP
phase=benchmark
if run_benchmark; then
  :
else
  status=$?
fi

sanitize_output
redaction_status=passed
if contains_prohibited_content "$sanitized_file"; then
  redaction_status=blocked
fi

if [ "$status" -ne 0 ]; then
  publish_failure "$phase" "$status" "$redaction_status"
  printf 'error: Gin adapter benchmark command failed with exit status %s\n' "$status" >&2
  exit "$status"
fi
if [ "$redaction_status" = blocked ]; then
  status=125
  publish_failure redaction "$status" "$redaction_status"
  printf 'error: prohibited content remained after benchmark redaction\n' >&2
  exit "$status"
fi

phase=parser
if ! python3 "$repo_root/scripts/parse-gin-adapter-benchmark.py" --input "$sanitized_file" --output "$results_file"; then
  status=125
  publish_failure "$phase" "$status" "$redaction_status"
  printf 'error: Gin adapter benchmark parser rejected the capture\n' >&2
  exit "$status"
fi

phase=chart
if ! run_chart_generation; then
  status=125
  sanitize_chart_output
  publish_failure "$phase" "$status" "$redaction_status"
  printf 'error: Gin adapter benchmark chart generation failed (reason=%s exit_status=%s)\n' \
    "$chart_failure_reason" "$chart_exit_status" >&2
  exit "$status"
fi

phase=publication
environment_publish=$private_dir/bench-environment-publish.txt
{
  printf 'capture_status: success\n'
  printf 'capture_stamp: %s\n' "$capture_stamp"
  printf 'chart_source: gin-adapter-benchmark-summary.vl.json\n'
  printf 'chart_svg: gin-adapter-benchmark-summary.svg\n'
  printf 'chart_png: gin-adapter-benchmark-summary.png\n'
  cat "$metadata_file"
} >"$environment_publish"

if ! stage_file "$sanitized_file" "$output_dir/bench-output.txt" || \
  ! stage_file "$results_file" "$output_dir/bench-results.json" || \
  ! stage_file "$environment_publish" "$output_dir/bench-environment.txt" || \
  ! stage_file "$chart_output_dir/gin-adapter-benchmark-summary.svg" "$chart_dir/gin-adapter-benchmark-summary.svg" false || \
  ! stage_file "$chart_output_dir/gin-adapter-benchmark-summary.png" "$chart_dir/gin-adapter-benchmark-summary.png" false || \
  ! stage_file "$chart_output_dir/gin-adapter-benchmark-summary.vl.json" "$chart_dir/gin-adapter-benchmark-summary.vl.json" false; then
  cleanup_staged
  status=125
  publish_failure "$phase" "$status" "$redaction_status"
  printf 'error: Gin adapter benchmark publication staging failed\n' >&2
  exit "$status"
fi

for pair in "${staged_files[@]}"; do
  target=${pair#*$'\t'}
  if ! prepare_publication_backup "$target"; then
    cleanup_staged
    status=125
    publish_failure "$phase" "$status" "$redaction_status"
    printf 'error: Gin adapter benchmark publication backup failed\n' >&2
    exit "$status"
  fi
done

publication_started=true
publish_status=0
for pair in "${staged_files[@]}"; do
  temporary=${pair%%$'\t'*}
  target=${pair#*$'\t'}
  if ! mv -f "$temporary" "$target"; then
    publish_status=125
    break
  fi
done
if [ "$publish_status" -ne 0 ]; then
  restore_status=0
  if ! restore_publication; then
    restore_status=125
  fi
  publication_started=false
  cleanup_staged
  status=125
  publish_failure "$phase" "$status" "$redaction_status"
  if [ "$restore_status" -ne 0 ]; then
    printf 'error: Gin adapter benchmark publication failed and rollback was incomplete\n' >&2
  else
    printf 'error: Gin adapter benchmark publication failed; canonical artifacts were rolled back\n' >&2
  fi
  exit "$status"
fi

printf 'captured %s/bench-results.json\n' "$output_dir"
printf 'generated %s/gin-adapter-benchmark-summary.png\n' "$chart_dir"
