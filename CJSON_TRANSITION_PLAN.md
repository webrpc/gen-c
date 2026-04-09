# cJSON Transition Plan

## Goal

Restore `cJSON` as the only JSON backend for `gen-c` and downstream `c-sdk`,
while keeping the recent refactor, correctness fixes, and implementation-size
improvements.

## Plan

1. Keep all recent refactor and quality improvements in `gen-c`.
   Preserve:
   - split template structure
   - required `null` / `any` decode fix
   - one-time curl init cleanup
   - prefix-aware header guards
   - reachability-based codec pruning
   - shallow method `prepare` / `parse` dedup

2. Replace only the JSON backend layer in `gen-c`.
   Move from direct `json-c` usage back to direct `cJSON` usage.
   Do not add a compatibility shim.
   Keep `bigint` encoded/decoded as JSON string.

3. Make removal of `json-c` a hard constraint.
   After this work, there should be:
   - no `json-c` dependency in `gen-c`
   - no `json-c` dependency in `c-sdk`
   - no generated code that references `json-c`
   - no leftover `json-c`-specific helpers, includes, docs, or build flags

4. Regenerate WAAS from the updated `gen-c`.
   Produce fresh `waas.gen.h` and `waas.gen.c`.
   Keep the benefits of the refactor and size optimizations.
   Reapply only the temporary downstream missing-`iss` tolerance patch if still
   needed.

5. Move `c-sdk` back to `cJSON` without losing the other integration work.
   Revert build/docs/formula/dependency changes from `json-c` to `cJSON`.
   Keep the updated WAAS generated client structure and all non-JSON-related
   improvements intact.

6. Validate both repos end to end.
   `gen-c`:
   - `go test ./...`
   - regenerate WAAS successfully
   - syntax-check generated C
   `c-sdk`:
   - configure
   - build
   - `ctest`

## Acceptance Criteria

- RTOS-friendly dependency story is restored with `cJSON`
- big numbers remain string-based
- all recent refactor / correctness / size wins remain in place
- no `json-c` artifacts remain in generator, generated code, or `c-sdk`
  integration
