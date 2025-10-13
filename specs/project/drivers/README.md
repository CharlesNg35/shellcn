# Driver Specs Index

Each file in this directory documents a single protocol driver. Use the template below when adding a new driver.

```
# <Driver Title>

## Overview
- Summary of the driver purpose.
- Driver type (native Go, Rust FFI, proxy).
- Maintainer.

## Descriptor Metadata
- `id`: <driver id>
- `title`: <display name>
- `category`: <terminal|desktop|container|database|object_storage|vm|network>
- `icon`: <Lucide icon name>
- `sort_order`: <int>

## Connection Schema
| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|

## Capability Flags
List of booleans surfaced by the driver (terminal, desktop, etc.)

## Permission Profile
- base: `{driver}.connect`
- manage: `{driver}.manage`
- feature scopes: `...`
- admin scopes: `...`

## Identity Requirements
- Credential vault keys or identity bindings.
- Input validation expectations.

## Frontend UX Notes
- Pages/components affected.
- Form layout suggestions.
- Capability specific UI toggles.

## Testing
- Unit tests, integration tests, fixtures.

## Future Enhancements
- Optional roadmap items.
```

Reference `../PROTOCOL_DRIVER_STANDARDS.md` for the full contract.
