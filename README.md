# gen-c

This repo contains the templates used by the `webrpc-gen` CLI to code-generate
webrpc C client code.

This generator, from a webrpc schema/design file, will code-generate:

1. Header output
   Public C enums, structs, support types, `init` / `free` helpers, client declarations,
   and lower-level request / response helpers.

2. Implementation output
   C JSON encode/decode helpers, generated method request / response handling,
   and an optional `libcurl`-based client runtime.

The generated client is intended to speak to any webrpc server language
(Go, nodejs, etc.) as long as the schema features used are supported by this target.

## Dependencies

Generated `header` output only depends on the C standard library headers included by
the generated file.

Generated `impl` output currently depends on:

- `cJSON`
- `libcurl`

The generated code targets C99.

Typical compile / link flags look like:

```bash
cc -std=c99 \
  $(pkg-config --cflags libcurl libcjson) \
  -c example.gen.c

cc -std=c99 \
  app.c example.gen.c \
  $(pkg-config --cflags --libs libcurl libcjson) \
  -o app
```

Dependency names can vary slightly by platform or package manager. The important part
is that the generated implementation can include `<cjson/cJSON.h>` and link against
`libcurl` and `cJSON`.

## Features

The current generator supports:

- client code generation
- separate `header` and `impl` emission
- generated DTO structs and enums
- generated `init` / `free` helpers for schema and method wrapper types
- `bigint` values encoded as JSON strings
- `timestamp` values encoded as JSON strings
- `any`, `null`, nested lists, nested maps, and nested structs
- map keys of type `string` and `enum`
- succinct method wire format
- generated lower-level helpers to:
  - prepare request bytes without sending them
  - send a prepared request with the generated transport
  - parse a raw HTTP response into generated response types
- generated `libcurl` client configuration for bearer auth, custom headers, and timeouts

## Limitations

The current generator does not support:

- server generation
- streaming methods
- map keys other than `string` or `enum`
- a shared external transport abstraction; the generated runtime is currently self-contained

Implementation generation also assumes a companion generated header include via
`-header=<file>`.

## Usage

Generate the header:

```bash
webrpc-gen \
  -schema=example.ridl \
  -target=./local-gen-c \
  -emit=header \
  -out=./example.gen.h
```

Generate the implementation:

```bash
webrpc-gen \
  -schema=example.ridl \
  -target=./local-gen-c \
  -emit=impl \
  -header=example.gen.h \
  -out=./example.gen.c
```

When published as a `gen-*` module, this can also be used via:

```bash
webrpc-gen \
  -schema=example.ridl \
  -target=github.com/webrpc/gen-c@<version> \
  -emit=header \
  -out=./example.gen.h
```

or:

```bash
webrpc-gen \
  -schema=example.ridl \
  -target=github.com/webrpc/gen-c@<version> \
  -emit=impl \
  -header=example.gen.h \
  -out=./example.gen.c
```

## Set Custom Template Variables

Change any of the following values by passing `-option="Value"` CLI flag to `webrpc-gen`.

| webrpc-gen -option | Description | Default value | Version |
| --- | --- | --- | --- |
| `-prefix=<name>` | symbol and type prefix | schema name in `snake_case` | v0.0.1 |
| `-client` | generate client declarations and runtime | `true` | v0.0.1 |
| `-emit=<mode>` | emit either `header` or `impl` | `header` | v0.0.1 |
| `-header=<file>` | header include used by `impl` output | `<prefix>.h` | v0.0.1 |

## Notes

- `-target` can be a local template directory or a git module path.
- `bigint` support is string-based by design to avoid precision loss in C JSON handling.
- The generated implementation is tested with smoke, codec, succinct, and reference interop coverage in this repo.
