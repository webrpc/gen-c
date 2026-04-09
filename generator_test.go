package c

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const webrpcGenModule = "github.com/webrpc/webrpc/cmd/webrpc-gen@v0.37.1"
const webrpcGenVersion = "v0.37.1"

func TestGenerateSmoke(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()

	header := filepath.Join(tmp, "smoke.gen.h")
	impl := filepath.Join(tmp, "smoke.gen.c")

	generateC(t, root, filepath.Join(root, "testdata", "smoke.ridl"), header, impl, "smoke")
	syntaxCheckHeader(t, header)
	syntaxCheckImpl(t, tmp, impl)
}

func TestGeneratedCodecBehavior(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()

	header := filepath.Join(tmp, "codec.gen.h")
	impl := filepath.Join(tmp, "codec.gen.c")
	generateC(t, root, filepath.Join(root, "testdata", "codec.ridl"), header, impl, "codec_test")

	testMain := filepath.Join(tmp, "codec_test_main.c")
	if err := os.WriteFile(testMain, []byte(codecTestProgram), 0o644); err != nil {
		t.Fatalf("write codec test program: %v", err)
	}

	cflags := pkgConfigFlags(t, "--cflags")
	libs := pkgConfigFlags(t, "--libs")

	bin := filepath.Join(tmp, "codec-test")
	args := append([]string{"-std=c99", "-Wall", "-Wextra"}, cflags...)
	args = append(args, "codec_test_main.c", "-o", bin)
	args = append(args, libs...)

	runCmd(t, tmp, "cc", args...)
	runCmd(t, tmp, bin)
}

func TestGeneratedSuccinctMethodBehavior(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()

	header := filepath.Join(tmp, "succinct.gen.h")
	impl := filepath.Join(tmp, "succinct.gen.c")
	generateC(t, root, filepath.Join(root, "testdata", "succinct.ridl"), header, impl, "succinct_test")

	testMain := filepath.Join(tmp, "succinct_test_main.c")
	if err := os.WriteFile(testMain, []byte(succinctTestProgram), 0o644); err != nil {
		t.Fatalf("write succinct test program: %v", err)
	}

	cflags := pkgConfigFlags(t, "--cflags")
	libs := pkgConfigFlags(t, "--libs")

	bin := filepath.Join(tmp, "succinct-test")
	args := append([]string{"-std=c99", "-Wall", "-Wextra"}, cflags...)
	args = append(args, "succinct_test_main.c", "-o", bin)
	args = append(args, libs...)

	runCmd(t, tmp, "cc", args...)
	runCmd(t, tmp, bin)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve repo root")
	}
	return filepath.Dir(file)
}

func generateC(t *testing.T, root, schemaPath, headerPath, implPath, prefix string) {
	t.Helper()

	baseArgs := []string{
		"run",
		"-ldflags=-X github.com/webrpc/webrpc.VERSION=" + webrpcGenVersion,
		webrpcGenModule,
		"-schema=" + schemaPath,
		"-target=" + root,
		"-prefix=" + prefix,
	}
	runCmd(t, root, "go", append(baseArgs, "-emit=header", "-out="+headerPath)...)
	runCmd(t, root, "go", append(baseArgs, "-emit=impl", "-header="+filepath.Base(headerPath), "-out="+implPath)...)
}

func syntaxCheckHeader(t *testing.T, headerPath string) {
	t.Helper()
	runCmd(t, filepath.Dir(headerPath), "cc", "-x", "c", "-std=c99", "-Wall", "-Wextra", "-fsyntax-only", filepath.Base(headerPath))
}

func syntaxCheckImpl(t *testing.T, workdir, implPath string) {
	t.Helper()
	args := []string{"-x", "c", "-std=c99", "-Wall", "-Wextra", "-fsyntax-only"}
	args = append(args, pkgConfigFlags(t, "--cflags")...)
	args = append(args, filepath.Base(implPath))
	runCmd(t, workdir, "cc", args...)
}

func pkgConfigFlags(t *testing.T, mode string) []string {
	t.Helper()

	candidates := [][]string{
		{mode, "libcjson", "libcurl"},
		{mode, "cjson", "libcurl"},
	}

	for _, args := range candidates {
		cmd := exec.Command("pkg-config", args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			return strings.Fields(stdout.String())
		}
	}

	t.Fatalf("pkg-config failed for cJSON/libcurl using mode %s", mode)
	return nil
}

func runCmd(t *testing.T, dir, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}

	return stdout.String()
}

const codecTestProgram = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "codec.gen.c"

static void fail_msg(const char *msg) {
    fprintf(stderr, "%s\n", msg);
    exit(1);
}

static void expect_true(int cond, const char *msg) {
    if (!cond) {
        fail_msg(msg);
    }
}

int main(void) {
    codec_test_payload value;
    codec_test_payload_init(&value);

    expect_true(codec_test_bigint_set_string(&value.count, "18446744073709551616") == 0, "count bigint set failed");
    value.explicit_any = codec_test_strdup("{\"k\":[1,true,null]}");
    expect_true(value.explicit_any != NULL, "explicit_any strdup failed");
    value.explicit_null = 1;

    value.nested = (codec_test_nested *)malloc(sizeof(*value.nested));
    expect_true(value.nested != NULL, "nested alloc failed");
    codec_test_nested_init(value.nested);
    expect_true(codec_test_bigint_set_string(&value.nested->id, "99") == 0, "nested bigint set failed");

    value.items.count = 2;
    value.items.items = (codec_test_bigint *)calloc(value.items.count, sizeof(*value.items.items));
    expect_true(value.items.items != NULL, "items alloc failed");
    expect_true(codec_test_bigint_set_string(&value.items.items[0], "7") == 0, "item 0 bigint set failed");
    expect_true(codec_test_bigint_set_string(&value.items.items[1], "8") == 0, "item 1 bigint set failed");

    cJSON *encoded = codec_test_payload_to_json(&value);
    expect_true(encoded != NULL, "encode failed");

    cJSON *count = cJSON_GetObjectItemCaseSensitive(encoded, "count");
    expect_true(cJSON_IsString(count), "count should encode as string");
    expect_true(strcmp(cJSON_GetStringValue(count), "18446744073709551616") == 0, "count bigint string mismatch");

    cJSON *explicit_any = cJSON_GetObjectItemCaseSensitive(encoded, "explicitAny");
    expect_true(cJSON_IsObject(explicit_any), "explicitAny should encode as object");
    expect_true(cJSON_IsArray(cJSON_GetObjectItemCaseSensitive(explicit_any, "k")), "explicitAny.k should be array");

    cJSON *explicit_null = cJSON_GetObjectItemCaseSensitive(encoded, "explicitNull");
    expect_true(cJSON_IsNull(explicit_null), "explicitNull should encode as null");

    cJSON *nested = cJSON_GetObjectItemCaseSensitive(encoded, "nested");
    cJSON *nested_id = cJSON_GetObjectItemCaseSensitive(nested, "id");
    expect_true(cJSON_IsString(nested_id), "nested.id should encode as string");
    expect_true(strcmp(cJSON_GetStringValue(nested_id), "99") == 0, "nested.id bigint string mismatch");

    cJSON *items = cJSON_GetObjectItemCaseSensitive(encoded, "items");
    expect_true(cJSON_IsArray(items), "items should encode as array");
    expect_true(cJSON_GetArraySize(items) == 2, "items length mismatch");
    expect_true(strcmp(cJSON_GetStringValue(cJSON_GetArrayItem(items, 0)), "7") == 0, "items[0] mismatch");
    expect_true(strcmp(cJSON_GetStringValue(cJSON_GetArrayItem(items, 1)), "8") == 0, "items[1] mismatch");

    cJSON_Delete(encoded);
    codec_test_payload_free(&value);

    {
        const char *json_text = "{\"count\":\"42\",\"explicitAny\":null,\"explicitNull\":null,\"maybeAny\":null,\"nested\":{\"id\":\"99\"},\"items\":[\"1\",\"2\"]}";
        cJSON *parsed = codec_test_cjson_parse(json_text);
        codec_test_payload decoded;
        codec_test_error error;

        expect_true(parsed != NULL, "parse decode JSON failed");
        codec_test_payload_init(&decoded);
        codec_test_error_init(&error);

        expect_true(codec_test_payload_from_json(parsed, &decoded, &error) == 0, "decode failed");
        expect_true(decoded.count.digits != NULL && strcmp(decoded.count.digits, "42") == 0, "decoded count mismatch");
        expect_true(decoded.explicit_any != NULL && strcmp(decoded.explicit_any, "null") == 0, "decoded explicitAny should preserve null");
        expect_true(decoded.has_maybe_any, "decoded maybeAny should mark field present");
        expect_true(decoded.maybe_any == NULL, "decoded maybeAny null should remain NULL payload");
        expect_true(decoded.nested != NULL, "decoded nested missing");
        expect_true(decoded.nested->id.digits != NULL && strcmp(decoded.nested->id.digits, "99") == 0, "decoded nested id mismatch");
        expect_true(decoded.items.count == 2, "decoded items length mismatch");
        expect_true(strcmp(decoded.items.items[0].digits, "1") == 0, "decoded items[0] mismatch");
        expect_true(strcmp(decoded.items.items[1].digits, "2") == 0, "decoded items[1] mismatch");

        codec_test_error_free(&error);
        codec_test_payload_free(&decoded);
        cJSON_Delete(parsed);
    }

    {
        const char *json_text = "{\"count\":\"42\",\"explicitAny\":null,\"nested\":{\"id\":\"99\"},\"items\":[]}";
        cJSON *parsed = codec_test_cjson_parse(json_text);
        codec_test_payload decoded;
        codec_test_error error;

        expect_true(parsed != NULL, "parse missing-required JSON failed");
        codec_test_payload_init(&decoded);
        codec_test_error_init(&error);

        expect_true(codec_test_payload_from_json(parsed, &decoded, &error) != 0, "decode should fail when required null field is missing");
        expect_true(error.message != NULL && strstr(error.message, "missing required field explicitNull") != NULL, "missing required field message mismatch");

        codec_test_error_free(&error);
        codec_test_payload_free(&decoded);
        cJSON_Delete(parsed);
    }

    return 0;
}
`

const succinctTestProgram = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "succinct.gen.c"

static void fail_msg(const char *msg) {
    fprintf(stderr, "%s\n", msg);
    exit(1);
}

static void expect_true(int cond, const char *msg) {
    if (!cond) {
        fail_msg(msg);
    }
}

int main(void) {
    succinct_test_demo_flatten_request request;
    succinct_test_demo_flatten_response response;
    cJSON *request_json = NULL;
    cJSON *response_json = NULL;
    succinct_test_error error;

    succinct_test_demo_flatten_request_init(&request);
    succinct_test_demo_flatten_response_init(&response);
    succinct_test_error_init(&error);

    request.flatten_request = (succinct_test_flatten_request *)malloc(sizeof(*request.flatten_request));
    expect_true(request.flatten_request != NULL, "request alloc failed");
    succinct_test_flatten_request_init(request.flatten_request);
    request.flatten_request->name = succinct_test_strdup("alice");
    expect_true(request.flatten_request->name != NULL, "request name alloc failed");
    request.flatten_request->amount = 42;

    request_json = succinct_test_demo_flatten_request_to_json(&request);
    expect_true(request_json != NULL, "succinct request encode failed");
    expect_true(cJSON_IsObject(request_json), "succinct request should encode to direct object");
    expect_true(cJSON_GetObjectItemCaseSensitive(request_json, "flattenRequest") == NULL, "succinct request must not wrap payload");
    expect_true(strcmp(cJSON_GetStringValue(cJSON_GetObjectItemCaseSensitive(request_json, "name")), "alice") == 0, "succinct request name mismatch");
    expect_true((uint64_t)cJSON_GetNumberValue(cJSON_GetObjectItemCaseSensitive(request_json, "amount")) == 42, "succinct request amount mismatch");

    response_json = cJSON_CreateObject();
    expect_true(response_json != NULL, "succinct response alloc failed");
    expect_true(cJSON_AddNumberToObject(response_json, "id", 99) != NULL, "succinct response id add failed");
    expect_true(cJSON_AddNumberToObject(response_json, "count", 7) != NULL, "succinct response count add failed");

    expect_true(succinct_test_demo_flatten_response_from_json(response_json, &response, &error) == 0, "succinct response decode failed");
    expect_true(response.flatten_response != NULL, "succinct response payload missing");
    expect_true(response.flatten_response->id == 99, "succinct response id mismatch");
    expect_true(response.flatten_response->count == 7, "succinct response count mismatch");

    cJSON_Delete(request_json);
    cJSON_Delete(response_json);
    succinct_test_demo_flatten_request_free(&request);
    succinct_test_demo_flatten_response_free(&response);
    succinct_test_error_free(&error);
    return 0;
}
`
