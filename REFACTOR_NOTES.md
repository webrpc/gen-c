# gen-c Refactor Notes

Current focus: split `impl.go.tmpl` into smaller template units without changing generated output.

Checklist:
- [x] Identify high-value seams in `impl.go.tmpl`
- [x] Extract runtime/JSON preamble into `implPreamble.go.tmpl`
- [x] Extract transport/error helpers into `implTransport.go.tmpl`
- [x] Extract struct JSON generation into `implStructJSON.go.tmpl`
- [x] Extract method JSON generation into `implMethodJSON.go.tmpl`
- [x] Extract client implementation into `implClient.go.tmpl`
- [x] Extract recursive JSON codec templates into `implJSONCodec.go.tmpl`
- [x] Reduce `impl.go.tmpl` to orchestration only
- [x] Review diff for behavior-preserving refactor only

Validation done:
- `go test ./...`
- generated WAAS impl with `webrpc-gen v0.36.0` and confirmed private struct codecs are pruned to reachable encode/decode paths
- syntax-checked generated WAAS impl with `cc -std=c99 -Wall -Wextra -fsyntax-only $(pkg-config --cflags libcjson || pkg-config --cflags cjson)`

Still worth doing later:
- add a template parsing/generation smoke test so refactors validate generated output, not just Go package compilation
- add `_examples` and a small interoperability check
