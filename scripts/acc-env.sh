#!/usr/bin/env bash
# Copyright (c) AO Studio
# SPDX-License-Identifier: MPL-2.0
#
# Keeps the acceptance-test environment in the macOS Keychain, one item per
# variable, and hands it to a single command rather than to the shell.
#
#   scripts/acc-env.sh set NAME [FILE]   store NAME, typed (hidden) or read from FILE
#   scripts/acc-env.sh unset NAME        remove NAME
#   scripts/acc-env.sh list              names stored, never values
#   scripts/acc-env.sh run CMD [ARG...]  run CMD with every stored variable set
#
# Values never appear on a command line, in a file in the repository, or in the
# output: they are base64-encoded (the private key is several lines, and a
# Keychain password is one) and passed to `security -i` on stdin rather than as
# an argument, where any process could read them from `ps`. `run` sets them
# only in the environment of CMD, so they do not outlive it.
#
# Only names this suite reads are accepted, so a typo fails here rather than
# being stored and silently ignored.

set -euo pipefail

# ACC_ENV_SERVICE exists so the script can be tested without touching real items.
SERVICE="${ACC_ENV_SERVICE:-terraform-provider-apple-acceptance}"

ALLOWED=(
  APPLE_APP_STORE_CONNECT_ISSUER_ID
  APPLE_APP_STORE_CONNECT_API_KEY
  APPLE_APP_STORE_CONNECT_PRIVATE_KEY
  APPLE_APP_STORE_CONNECT_MAX_RETRIES
  APPLE_APP_STORE_CONNECT_REQUEST_TIMEOUT
  APPLE_TEST_APP_ID
  APPLE_TEST_BUNDLE_ID
  APPLE_TEST_BUILD_NUMBER
  APPLE_TEST_BUILD_PRE_RELEASE_VERSION
  APPLE_TEST_BETA_TESTER_EMAIL
  APPLE_TEST_USER_EMAIL
  APPLE_TEST_USER_INVITE_EMAIL
  APPLE_TEST_ALLOW_APP_PRICING
  APPLE_TEST_ALLOW_USER_REMOVAL
  APPLE_TEST_CI_PRODUCT_ID
  APPLE_TEST_CI_REPOSITORY_ID
  APPLE_TEST_CI_XCODE_VERSION_ID
  APPLE_TEST_CI_MAC_OS_VERSION_ID
  APPLE_TEST_CI_SCHEME
  APPLE_TEST_CI_CONTAINER_FILE_PATH
)

die() {
  echo "acc-env: $*" >&2
  exit 1
}

usage() {
  sed -n '6,11p' "$0" | sed 's/^# \{0,1\}//' >&2
  exit 2
}

[ "$(uname -s)" = "Darwin" ] || die "the Keychain is macOS only; export the variables another way"
command -v security >/dev/null || die "/usr/bin/security not found"

allowed() {
  local name
  for name in "${ALLOWED[@]}"; do
    [ "$name" = "$1" ] && return 0
  done
  return 1
}

check_name() {
  allowed "$1" || die "$1 is not a variable the acceptance tests read (see ALLOWED in $0)"
}

# lookup prints the stored value of $1, or fails if there is none.
lookup() {
  local encoded
  encoded="$(security find-generic-password -s "$SERVICE" -a "$1" -w 2>/dev/null)" || return 1
  printf '%s' "$encoded" | base64 -D
}

cmd_set() {
  [ $# -ge 1 ] && [ $# -le 2 ] || usage
  local name="$1" value
  check_name "$name"

  if [ $# -eq 2 ]; then
    [ -r "$2" ] || die "cannot read $2"
    value="$(cat "$2")"
  else
    [ -t 0 ] || die "type the value at a terminal, or pass a file"
    read -r -s -p "$name: " value
    echo >&2
  fi
  [ -n "$value" ] || die "$name is empty; nothing stored"

  local encoded
  encoded="$(printf '%s' "$value" | base64 | tr -d '\n')"
  # -U updates an existing item. Fed on stdin, so the value is never an argv.
  printf 'add-generic-password -U -s %s -a %s -w %s\n' "$SERVICE" "$name" "$encoded" | security -i >/dev/null \
    || die "could not store $name"
  echo "stored $name" >&2
}

cmd_unset() {
  [ $# -eq 1 ] || usage
  check_name "$1"
  security delete-generic-password -s "$SERVICE" -a "$1" >/dev/null 2>&1 || die "$1 is not stored"
  echo "removed $1" >&2
}

cmd_list() {
  local name
  for name in "${ALLOWED[@]}"; do
    if security find-generic-password -s "$SERVICE" -a "$name" >/dev/null 2>&1; then
      echo "$name"
    fi
  done
}

cmd_run() {
  [ $# -ge 1 ] || usage
  local name value count=0
  for name in "${ALLOWED[@]}"; do
    if value="$(lookup "$name")"; then
      export "$name=$value"
      count=$((count + 1))
    fi
  done
  [ "$count" -gt 0 ] || die "nothing stored under $SERVICE; start with: $0 set APPLE_APP_STORE_CONNECT_ISSUER_ID"
  echo "acc-env: $count variable(s) loaded from the Keychain" >&2
  exec "$@"
}

[ $# -ge 1 ] || usage
sub="$1"
shift
case "$sub" in
  set) cmd_set "$@" ;;
  unset) cmd_unset "$@" ;;
  list) cmd_list "$@" ;;
  run) cmd_run "$@" ;;
  *) usage ;;
esac
