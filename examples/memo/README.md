# memo — reference out-of-tree ShellCN plugin

An in-memory notes store, kept deliberately small to show the whole authoring
surface: a declarative manifest (a table panel with a create form and a row
delete), unary routes (`list`/`create`/`delete`), and per-connection session
state. It is its **own Go module** and depends only on
`github.com/charlesng35/shellcn/sdk` — no core, no `internal/`.

## Build

```sh
go build -o memo .
# cross-compile for the gateway's host, e.g.:
GOOS=linux  GOARCH=amd64 go build -o memo-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o memo-darwin-arm64 .
```

A plugin is a normal Go binary, so it is OS/arch-specific — build for the machine
the gateway runs on. Prefer pure-Go dependencies so cross-compilation stays a
one-liner.

## Install

Drop the binary into the gateway's plugins directory and restart:

```sh
cp memo /path/to/shellcn/plugins.d/
```

`memo` then appears in the connection catalog with **no core changes**.

## Out-of-tree note

This example uses a `replace` to the in-repo SDK. A real third-party plugin
removes that line and pins the published module instead:

```sh
go get github.com/charlesng35/shellcn/sdk@latest
```

See `docs/external-plugins.md` for the full authoring guide (streaming, channels,
egress through `cfg.Net`, the HTTP proxy, the trust model, and versioning).
