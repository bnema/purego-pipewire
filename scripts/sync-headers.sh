#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' 'sync-headers is not implemented yet in cmd/pipewiregen.' >&2
printf '%s\n' 'This script is a maintainer placeholder for the planned header-sync workflow.' >&2
exit 1

go run ./cmd/pipewiregen --sync-headers \
	--pipewire-include "${PIPEWIRE_INCLUDE_DIR:-/usr/include/pipewire-0.3}" \
	--spa-include "${SPA_INCLUDE_DIR:-/usr/include/spa-0.2}" \
	--out-dir gen
