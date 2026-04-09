# pipewiregen

The `pipewiregen` tool generates Go bindings for PipeWire and SPA libraries from JSON model files.

## Usage

### Generate bindings from checked-in models

    go run ./cmd/pipewiregen

This reads `gen/pipewire.json` and `gen/spa.json`, then emits:
- `internal/capi/*_gen.go` — raw function types and registration helpers
- `internal/ports/out/*_gen.go` — outbound port interfaces

### Sync headers from system includes (maintainers)

    ./scripts/sync-headers.sh

Or run directly:

    go run ./cmd/pipewiregen --sync-headers \
        --pipewire-include /usr/include/pipewire-0.3 \
        --spa-include /usr/include/spa-0.2 \
        --out-dir gen

## Model format

The JSON model files follow this structure:

```json
{
  "libraries": [
    {"name": "pipewire", "soname": "libpipewire-0.3.so.0"}
  ],
  "groups": [
    {"name": "init", "interface": "InitAPI", "package": "out", "symbols": ["pw_init", "pw_deinit"]}
  ],
  "symbols": [
    {"name": "pw_init", "library": "pipewire", "group": "init", "signature": "func(argc *int32, argv ***byte)"}
  ]
}
```

- `libraries`: Shared libraries to load
- `groups`: Symbol groupings that become Go interfaces
- `symbols`: Individual function symbols with their Go signatures

## Architecture

The generator follows a pipeline design:

1. **parser**: Loads and validates JSON model files
2. **model**: Core data structures for libraries, groups, and symbols
3. **normalize**: PipeWire-specific naming conventions
4. **emitter**: Renders Go code from templates

See the individual package directories for implementation details.
