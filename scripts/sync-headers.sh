#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/pipewiregen --sync-headers \
	--pipewire-include "${PIPEWIRE_INCLUDE_DIR:-/usr/include/pipewire-0.3}" \
	--spa-include "${SPA_INCLUDE_DIR:-/usr/include/spa-0.2}" \
	--out-dir gen
