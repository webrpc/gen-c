package c

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestInteropWithWebrpcTest(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()

	webrpcTest := ensureWebrpcTestBinary(t)
	toolDir := filepath.Dir(webrpcTest)

	schemaText := runCmdOutputEnv(t, root, withPrependedPath(os.Environ(), toolDir), webrpcTest, "-print-schema")

	schemaPath := filepath.Join(tmp, "interop.ridl")
	if err := os.WriteFile(schemaPath, []byte(schemaText), 0o644); err != nil {
		t.Fatalf("write interop schema: %v", err)
	}

	header := filepath.Join(tmp, "interop.gen.h")
	impl := filepath.Join(tmp, "interop.gen.c")
	generateC(t, root, schemaPath, header, impl, "test")

	testMain := filepath.Join(tmp, "interop_test_main.c")
	if err := os.WriteFile(testMain, []byte(interopCTestProgram), 0o644); err != nil {
		t.Fatalf("write interop C test program: %v", err)
	}

	cflags := pkgConfigFlags(t, "--cflags")
	libs := pkgConfigFlags(t, "--libs")
	bin := filepath.Join(tmp, "interop-test")
	args := append([]string{"-std=c99", "-Wall", "-Wextra"}, cflags...)
	args = append(args, "interop_test_main.c", "-o", bin)
	args = append(args, libs...)
	runCmd(t, tmp, "cc", args...)

	port := freeTCPPort(t)
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	serverCmd := exec.Command(webrpcTest, "-server", "-port="+strconv.Itoa(port), "-timeout=10m")
	serverCmd.Env = withPrependedPath(os.Environ(), toolDir)
	var serverLog bytes.Buffer
	serverCmd.Stdout = &serverLog
	serverCmd.Stderr = &serverLog

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start webrpc-test server: %v", err)
	}
	t.Cleanup(func() {
		if serverCmd.Process != nil {
			_ = serverCmd.Process.Kill()
		}
		_ = serverCmd.Wait()
	})

	waitForTCP(t, port, 10*time.Second, &serverLog)
	runCmd(t, tmp, bin, serverURL)
}

func ensureWebrpcTestBinary(t *testing.T) string {
	t.Helper()

	cacheDir := filepath.Join(os.TempDir(), "gen-c-webrpc-bin", webrpcGenVersion, runtime.GOOS+"-"+runtime.GOARCH)
	name := "webrpc-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binPath := filepath.Join(cacheDir, name)

	if info, err := os.Stat(binPath); err == nil && info.Mode().IsRegular() {
		return binPath
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("create webrpc-test cache dir: %v", err)
	}

	if url, ok := webrpcTestReleaseURL(); ok {
		if err := downloadExecutable(url, binPath); err == nil {
			return binPath
		} else {
			t.Logf("download webrpc-test binary failed, falling back to go install: %v", err)
		}
	}

	env := append(os.Environ(), "GOWORK=off", "GOBIN="+cacheDir)
	runCmdEnv(t, cacheDir, env, "go", "install", "github.com/webrpc/webrpc/cmd/webrpc-test@"+webrpcGenVersion)
	return binPath
}

func webrpcTestReleaseURL() (string, bool) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "darwin", "linux":
	default:
		return "", false
	}

	switch goarch {
	case "amd64", "arm64":
	default:
		return "", false
	}

	return fmt.Sprintf(
		"https://github.com/webrpc/webrpc/releases/download/%s/webrpc-test.%s-%s",
		webrpcGenVersion,
		goos,
		goarch,
	), true
}

func downloadExecutable(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dst)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer ln.Close()

	return ln.Addr().(*net.TCPAddr).Port
}

func waitForTCP(t *testing.T, port int, timeout time.Duration, serverLog *bytes.Buffer) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("webrpc-test server did not become ready on %s\nserver output:\n%s", address, serverLog.String())
}

func withPrependedPath(env []string, dir string) []string {
	pathValue := dir
	if current, ok := lookupEnv(env, "PATH"); ok && current != "" {
		pathValue = dir + string(os.PathListSeparator) + current
	}
	return upsertEnv(env, "PATH", pathValue)
}

func lookupEnv(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix), true
		}
	}
	return "", false
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			result = append(result, prefix+value)
			found = true
			continue
		}
		result = append(result, entry)
	}
	if !found {
		result = append(result, prefix+value)
	}
	return result
}

func runCmdEnv(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}

	return stdout.String()
}

func runCmdOutputEnv(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	return runCmdEnv(t, dir, env, name, args...)
}

const interopCTestProgram = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "interop.gen.c"

static void fail_msg(const char *msg) {
    fprintf(stderr, "%s\n", msg);
    exit(1);
}

static void expect_true(int cond, const char *msg) {
    if (!cond) {
        fail_msg(msg);
    }
}

int main(int argc, char **argv) {
    const char *base_url;
    test_test_api_client *client;
    test_error error;

    if (argc != 2) {
        fail_msg("expected base URL argument");
    }

    base_url = argv[1];
    client = test_test_api_client_create(base_url, NULL);
    expect_true(client != NULL, "failed to create client");

    test_error_init(&error);

    {
        test_test_api_get_empty_request request;
        test_test_api_get_empty_response response;
        test_test_api_get_empty_request_init(&request);
        test_test_api_get_empty_response_init(&response);
        expect_true(test_test_api_get_empty(client, &request, &response, &error) == 0, "GetEmpty failed");
        test_test_api_get_empty_response_free(&response);
    }

    {
        test_test_api_get_error_request request;
        test_test_api_get_error_response response;
        test_test_api_get_error_request_init(&request);
        test_test_api_get_error_response_init(&response);
        expect_true(test_test_api_get_error(client, &request, &response, &error) != 0, "GetError should fail");
        expect_true(error.code == 0, "GetError code mismatch");
        expect_true(error.http_status == 400, "GetError HTTP status mismatch");
        expect_true(error.name != NULL && strcmp(error.name, "WebrpcEndpoint") == 0, "GetError name mismatch");
        expect_true(error.message != NULL && strcmp(error.message, "endpoint error") == 0, "GetError message mismatch");
        test_error_free(&error);
        test_error_init(&error);
        test_test_api_get_error_response_free(&response);
    }

    {
        test_test_api_get_one_request get_request;
        test_test_api_get_one_response get_response;
        test_test_api_send_one_request send_request;
        test_test_api_send_one_response send_response;

        test_test_api_get_one_request_init(&get_request);
        test_test_api_get_one_response_init(&get_response);
        test_test_api_send_one_response_init(&send_response);

        expect_true(test_test_api_get_one(client, &get_request, &get_response, &error) == 0, "GetOne failed");
        expect_true(get_response.one != NULL, "GetOne payload missing");
        expect_true(get_response.one->id == 1, "GetOne id mismatch");
        expect_true(get_response.one->name != NULL && strcmp(get_response.one->name, "one") == 0, "GetOne name mismatch");

        test_test_api_send_one_request_init(&send_request);
        send_request.one = get_response.one;
        expect_true(test_test_api_send_one(client, &send_request, &send_response, &error) == 0, "SendOne failed");

        test_test_api_send_one_response_free(&send_response);
        test_test_api_get_one_response_free(&get_response);
    }

    {
        test_test_api_get_multi_request get_request;
        test_test_api_get_multi_response get_response;
        test_test_api_send_multi_request send_request;
        test_test_api_send_multi_response send_response;

        test_test_api_get_multi_request_init(&get_request);
        test_test_api_get_multi_response_init(&get_response);
        test_test_api_send_multi_response_init(&send_response);

        expect_true(test_test_api_get_multi(client, &get_request, &get_response, &error) == 0, "GetMulti failed");
        expect_true(get_response.one != NULL && get_response.two != NULL && get_response.three != NULL, "GetMulti payload missing");
        expect_true(strcmp(get_response.one->name, "one") == 0, "GetMulti one mismatch");
        expect_true(strcmp(get_response.two->name, "two") == 0, "GetMulti two mismatch");
        expect_true(strcmp(get_response.three->name, "three") == 0, "GetMulti three mismatch");

        test_test_api_send_multi_request_init(&send_request);
        send_request.one = get_response.one;
        send_request.two = get_response.two;
        send_request.three = get_response.three;
        expect_true(test_test_api_send_multi(client, &send_request, &send_response, &error) == 0, "SendMulti failed");

        test_test_api_send_multi_response_free(&send_response);
        test_test_api_get_multi_response_free(&get_response);
    }

    {
        test_test_api_get_complex_request get_request;
        test_test_api_get_complex_response get_response;
        test_test_api_send_complex_request send_request;
        test_test_api_send_complex_response send_response;
        size_t i;
        int found_read = 0;
        int found_write = 0;

        test_test_api_get_complex_request_init(&get_request);
        test_test_api_get_complex_response_init(&get_response);
        test_test_api_send_complex_response_init(&send_response);

        expect_true(test_test_api_get_complex(client, &get_request, &get_response, &error) == 0, "GetComplex failed");
        expect_true(get_response.complex != NULL, "GetComplex payload missing");

        expect_true(get_response.complex->meta.count == 2, "GetComplex meta count mismatch");
        expect_true(get_response.complex->meta_nested_example.count == 1, "GetComplex nested meta count mismatch");
        expect_true(get_response.complex->names_list.count == 3, "GetComplex names list count mismatch");
        expect_true(strcmp(get_response.complex->names_list.items[0], "John") == 0, "GetComplex names list item 0 mismatch");
        expect_true(strcmp(get_response.complex->names_list.items[1], "Alice") == 0, "GetComplex names list item 1 mismatch");
        expect_true(strcmp(get_response.complex->names_list.items[2], "Jakob") == 0, "GetComplex names list item 2 mismatch");
        expect_true(get_response.complex->nums_list.count == 4, "GetComplex nums list count mismatch");
        expect_true(get_response.complex->nums_list.items[3] == 4534643543LL, "GetComplex nums list item mismatch");
        expect_true(get_response.complex->double_array.count == 2, "GetComplex double array outer count mismatch");
        expect_true(get_response.complex->double_array.items[0].count == 1, "GetComplex double array inner count mismatch");
        expect_true(strcmp(get_response.complex->double_array.items[0].items[0], "testing") == 0, "GetComplex double array first value mismatch");
        expect_true(strcmp(get_response.complex->double_array.items[1].items[0], "api") == 0, "GetComplex double array second value mismatch");
        expect_true(get_response.complex->list_of_maps.count == 1, "GetComplex list_of_maps count mismatch");
        expect_true(get_response.complex->list_of_maps.items[0].count == 3, "GetComplex list_of_maps entry count mismatch");
        expect_true(get_response.complex->list_of_users.count == 1, "GetComplex list_of_users count mismatch");
        expect_true(get_response.complex->list_of_users.items[0] != NULL, "GetComplex list_of_users item missing");
        expect_true(get_response.complex->list_of_users.items[0]->id == 1, "GetComplex list_of_users id mismatch");
        expect_true(strcmp(get_response.complex->list_of_users.items[0]->username, "John-Doe") == 0, "GetComplex list_of_users username mismatch");
        expect_true(strcmp(get_response.complex->list_of_users.items[0]->role, "admin") == 0, "GetComplex list_of_users role mismatch");
        expect_true(get_response.complex->map_of_users.count == 1, "GetComplex map_of_users count mismatch");
        expect_true(get_response.complex->user != NULL, "GetComplex user missing");
        expect_true(get_response.complex->user->id == 1, "GetComplex user id mismatch");
        expect_true(strcmp(get_response.complex->user->username, "John-Doe") == 0, "GetComplex user username mismatch");
        expect_true(strcmp(get_response.complex->user->role, "admin") == 0, "GetComplex user role mismatch");
        expect_true(get_response.complex->status == TEST_STATUS_AVAILABLE, "GetComplex status mismatch");

        for (i = 0; i < get_response.complex->list_of_maps.items[0].count; ++i) {
            const char *key = get_response.complex->list_of_maps.items[0].keys[i];
            uint32_t value = get_response.complex->list_of_maps.items[0].values[i];
            if (strcmp(key, "john") == 0) {
                expect_true(value == 1, "GetComplex list_of_maps john mismatch");
            } else if (strcmp(key, "alice") == 0) {
                expect_true(value == 2, "GetComplex list_of_maps alice mismatch");
            } else if (strcmp(key, "Jakob") == 0) {
                expect_true(value == 251, "GetComplex list_of_maps Jakob mismatch");
            } else {
                fail_msg("GetComplex list_of_maps unexpected key");
            }
        }

        for (i = 0; i < get_response.complex->map_of_users.count; ++i) {
            const char *key = get_response.complex->map_of_users.keys[i];
            test_user *value = get_response.complex->map_of_users.values[i];
            if (strcmp(key, "admin") != 0) {
                fail_msg("GetComplex map_of_users unexpected key");
            }
            expect_true(value != NULL, "GetComplex map_of_users value missing");
            expect_true(value->id == 1, "GetComplex map_of_users id mismatch");
            expect_true(strcmp(value->username, "John-Doe") == 0, "GetComplex map_of_users username mismatch");
            expect_true(strcmp(value->role, "admin") == 0, "GetComplex map_of_users role mismatch");
        }

        test_test_api_send_complex_request_init(&send_request);
        send_request.complex = get_response.complex;
        expect_true(test_test_api_send_complex(client, &send_request, &send_response, &error) == 0, "SendComplex failed");

        test_test_api_send_complex_response_free(&send_response);
        test_test_api_get_complex_response_free(&get_response);

        {
            test_test_api_get_enum_map_request request;
            test_test_api_get_enum_map_response response;

            test_test_api_get_enum_map_request_init(&request);
            test_test_api_get_enum_map_response_init(&response);

            expect_true(test_test_api_get_enum_map(client, &request, &response, &error) == 0, "GetEnumMap failed");
            expect_true(response.map.count == 2, "GetEnumMap count mismatch");
            for (i = 0; i < response.map.count; ++i) {
                if (response.map.keys[i] == TEST_ACCESS_READ) {
                    expect_true(response.map.values[i] == 1, "GetEnumMap READ mismatch");
                    found_read = 1;
                } else if (response.map.keys[i] == TEST_ACCESS_WRITE) {
                    expect_true(response.map.values[i] == 2, "GetEnumMap WRITE mismatch");
                    found_write = 1;
                } else {
                    fail_msg("GetEnumMap unexpected key");
                }
            }
            expect_true(found_read, "GetEnumMap missing READ");
            expect_true(found_write, "GetEnumMap missing WRITE");

            test_test_api_get_enum_map_response_free(&response);
        }
    }

    {
        test_test_api_get_enum_list_request request;
        test_test_api_get_enum_list_response response;

        test_test_api_get_enum_list_request_init(&request);
        test_test_api_get_enum_list_response_init(&response);

        expect_true(test_test_api_get_enum_list(client, &request, &response, &error) == 0, "GetEnumList failed");
        expect_true(response.list.count == 2, "GetEnumList length mismatch");
        expect_true(response.list.items[0] == TEST_STATUS_AVAILABLE, "GetEnumList item 0 mismatch");
        expect_true(response.list.items[1] == TEST_STATUS_NOT_AVAILABLE, "GetEnumList item 1 mismatch");

        test_test_api_get_enum_list_response_free(&response);
    }

    {
        test_test_api_get_schema_error_request request;
        test_test_api_get_schema_error_response response;

        test_test_api_get_schema_error_request_init(&request);
        test_test_api_get_schema_error_response_init(&response);
        request.code = 100;

        expect_true(test_test_api_get_schema_error(client, &request, &response, &error) != 0, "GetSchemaError should fail");
        expect_true(error.code == 100, "GetSchemaError code mismatch");
        expect_true(error.http_status == 429, "GetSchemaError HTTP status mismatch");
        expect_true(error.name != NULL && strcmp(error.name, "RateLimited") == 0, "GetSchemaError name mismatch");
        expect_true(error.message != NULL && strcmp(error.message, "too many requests") == 0, "GetSchemaError message mismatch");
        expect_true(error.cause != NULL && strcmp(error.cause, "1000 req/min exceeded") == 0, "GetSchemaError cause mismatch");

        test_error_free(&error);
        test_error_init(&error);
        test_test_api_get_schema_error_response_free(&response);
    }

    test_error_free(&error);
    test_test_api_client_destroy(client);
    return 0;
}
`
