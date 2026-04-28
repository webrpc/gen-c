# gen-c

This repo contains the templates used by the `webrpc-gen` CLI to code-generate
webrpc C client code.

This generator, from a webrpc schema/design file, will code-generate:

1. Header output
   Public C enums, structs, support types, `init` / `free` helpers, client declarations,
   and lower-level request / response helpers.

2. Implementation output
   C JSON encode/decode helpers, generated method request / response handling,
   and an optional self-contained `libcurl`-based HTTP transport/runtime.

The generated client is intended to speak to any webrpc server language
(Go, nodejs, etc.) as long as the schema features used are supported by this target.

## Dependencies

Generated `header` output only depends on the C standard library headers included by
the generated file.

Generated `impl` output depends on:

- `cJSON`
- `libcurl`, unless the generated implementation is compiled with the
  prefix-based no-curl guard

The generated code targets C99.

When using generated implementation output that sends requests through the
generated `libcurl` transport, call the generated runtime hooks before the
first request and after the last one. For example, if you generate with
`-prefix=example`, call `example_runtime_init()` before the first request and
`example_runtime_cleanup()` after the last one. This follows libcurl's
documented global initialization model:
[libcurl API overview](https://curl.se/libcurl/c/libcurl.html).

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

To build generated implementation output without the built-in libcurl transport, define
`<PREFIX>_NO_CURL_TRANSPORT`, where `<PREFIX>` is the generated prefix uppercased. For
example, code generated with `-prefix=example` can be compiled with:

```bash
cc -std=c99 \
  -DEXAMPLE_NO_CURL_TRANSPORT \
  $(pkg-config --cflags libcjson) \
  -c example.gen.c

cc -std=c99 \
  -DEXAMPLE_NO_CURL_TRANSPORT \
  app.c example.gen.c \
  $(pkg-config --cflags --libs libcjson) \
  -o app
```

No-curl mode removes the generated implementation's libcurl include, types, and link
dependency. It does not remove the `cJSON` dependency because generated JSON
encode/decode and response parsing still use `cJSON`.

The lower-level request/response helpers remain available in no-curl mode:
`example_<service>_<method>_prepare_request(...)` builds a prepared request and
`example_<service>_<method>_parse_response(...)` parses an HTTP response supplied by
your own transport. Runtime and client functions still link; send attempts fail with a
`TransportError` indicating that the built-in curl transport is disabled.

Because the generated implementation uses `cJSON`, exact large 64-bit integer handling
follows `cJSON`'s numeric behavior. If your API needs exact integer round-tripping beyond
normal JSON number precision expectations, prefer `bigint` in the schema instead of
`int64` / `uint64`.

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
- generated client configuration for bearer auth, custom headers, timeouts, and bounded response buffering

## Limitations

The current generator does not support:

- server generation
- streaming methods
- map keys other than `string` or `enum`
- a shared external transport abstraction; the generated runtime is currently self-contained
- automatic redirect following

Generated client options include `max_response_bytes`, which bounds the response body
buffer used by the built-in curl transport. `*_client_options_init(...)` defaults this
to `1024 * 1024` bytes. Leaving it zero is treated the same as the default, not as an
unlimited response size.

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
- for precision-sensitive large integer values, prefer `bigint` over `int64` / `uint64` in the schema when targeting this generator
- The generated implementation is tested with smoke, codec, succinct, and reference interop coverage in this repo.
