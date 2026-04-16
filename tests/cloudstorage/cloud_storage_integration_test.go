// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudstorage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/googleapis/mcp-toolbox/internal/testutils"
	"github.com/googleapis/mcp-toolbox/tests"
	"google.golang.org/api/option"
)

var (
	CloudStorageSourceType = "cloud-storage"
	CloudStorageProject    = os.Getenv("CLOUD_STORAGE_PROJECT")
)

const (
	helloObject = "seed/hello.txt"
	jsonObject  = "seed/nested/data.json"
	helloBody   = "hello world"
	jsonBody    = `{"foo":"bar"}`
)

func getCloudStorageVars(t *testing.T) map[string]any {
	if CloudStorageProject == "" {
		t.Fatal("'CLOUD_STORAGE_PROJECT' not set")
	}
	return map[string]any{
		"type":    CloudStorageSourceType,
		"project": CloudStorageProject,
	}
}

func initStorageClient(ctx context.Context) (*storage.Client, error) {
	return storage.NewClient(ctx, option.WithUserAgent("genai-toolbox-integration-test"))
}

func TestCloudStorageToolEndpoints(t *testing.T) {
	sourceConfig := getCloudStorageVars(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := initStorageClient(ctx)
	if err != nil {
		t.Fatalf("unable to create Cloud Storage client: %s", err)
	}
	defer client.Close()

	// Bucket names must be globally unique and match [a-z0-9_.-]{3,63}.
	bucketName := "toolbox-it-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:20]
	t.Logf("Using test bucket %q", bucketName)

	teardown := setupCloudStorageTestData(t, ctx, client, CloudStorageProject, bucketName)
	defer teardown(t)

	toolsFile := getCloudStorageToolsConfig(sourceConfig)

	cmd, cleanup, err := tests.StartCmd(ctx, toolsFile, "--enable-api")
	if err != nil {
		t.Fatalf("command initialization returned an error: %s", err)
	}
	defer cleanup()

	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()
	out, err := testutils.WaitForString(waitCtx, regexp.MustCompile(`Server ready to serve`), cmd.Out)
	if err != nil {
		t.Logf("toolbox command logs: \n%s", out)
		t.Fatalf("toolbox didn't start successfully: %s", err)
	}

	runCloudStorageToolGetTest(t)
	runCloudStorageListObjectsTest(t, bucketName)
	runCloudStorageReadObjectTest(t, bucketName)
}

func getCloudStorageToolsConfig(sourceConfig map[string]any) map[string]any {
	return map[string]any{
		"sources": map[string]any{
			"my-instance": sourceConfig,
		},
		"tools": map[string]any{
			"my-list-objects": map[string]any{
				"type":        "cloud-storage-list-objects",
				"source":      "my-instance",
				"description": "List objects in a Cloud Storage bucket.",
			},
			"my-read-object": map[string]any{
				"type":        "cloud-storage-read-object",
				"source":      "my-instance",
				"description": "Read a Cloud Storage object.",
			},
		},
	}
}

func setupCloudStorageTestData(t *testing.T, ctx context.Context, client *storage.Client, project, bucket string) func(*testing.T) {
	bkt := client.Bucket(bucket)
	if err := bkt.Create(ctx, project, &storage.BucketAttrs{Location: "US"}); err != nil {
		t.Fatalf("failed to create bucket %q: %v", bucket, err)
	}

	writeSeed := func(name, contentType, body string) {
		w := bkt.Object(name).NewWriter(ctx)
		w.ContentType = contentType
		if _, err := io.WriteString(w, body); err != nil {
			_ = w.Close()
			t.Fatalf("failed to write seed object %q: %v", name, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("failed to close writer for seed object %q: %v", name, err)
		}
	}

	writeSeed(helloObject, "text/plain", helloBody)
	writeSeed(jsonObject, "application/json", jsonBody)

	return func(t *testing.T) {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		it := bkt.Objects(cleanupCtx, nil)
		for {
			attrs, err := it.Next()
			if err != nil {
				// iterator.Done or another error: stop iterating and attempt bucket delete.
				break
			}
			if delErr := bkt.Object(attrs.Name).Delete(cleanupCtx); delErr != nil {
				t.Logf("cleanup: failed to delete object %q: %v", attrs.Name, delErr)
			}
		}
		if err := bkt.Delete(cleanupCtx); err != nil {
			t.Logf("cleanup: failed to delete bucket %q: %v", bucket, err)
		}
	}
}

// invokeTool POSTs to the tool invoke endpoint and returns the parsed `result`
// string (which is itself a JSON-encoded payload). On non-200 responses, the
// full body is returned as the error.
func invokeTool(t *testing.T, toolName, requestBody string) (string, int) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:5000/api/tool/%s/invoke", toolName)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(requestBody))
	if err != nil {
		t.Fatalf("unable to create request: %s", err)
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unable to send request: %s", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return string(bodyBytes), resp.StatusCode
	}
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("failed to parse response JSON: %s (body=%s)", err, string(bodyBytes))
	}
	result, _ := body["result"].(string)
	return result, resp.StatusCode
}

func runCloudStorageToolGetTest(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:5000/api/tool/my-list-objects/")
	if err != nil {
		t.Fatalf("error when sending a request: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("response status code is not 200: got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("error parsing response body: %s", err)
	}
	toolsRaw, ok := body["tools"].(map[string]any)
	if !ok {
		t.Fatalf("unable to find tools in response body: %v", body)
	}
	toolInfo, ok := toolsRaw["my-list-objects"].(map[string]any)
	if !ok {
		t.Fatalf("my-list-objects missing from tools response: %v", toolsRaw)
	}
	params, ok := toolInfo["parameters"].([]any)
	if !ok {
		t.Fatalf("parameters missing or wrong type: %v", toolInfo)
	}
	if len(params) != 5 {
		t.Fatalf("expected 5 parameters, got %d: %v", len(params), params)
	}
	// First parameter should be 'bucket', required.
	first, _ := params[0].(map[string]any)
	if first["name"] != "bucket" {
		t.Fatalf("expected first parameter to be 'bucket', got %v", first["name"])
	}
	if required, _ := first["required"].(bool); !required {
		t.Fatalf("expected 'bucket' parameter to be required, got %v", first)
	}
}

func runCloudStorageListObjectsTest(t *testing.T, bucket string) {
	t.Run("list with prefix", func(t *testing.T) {
		result, status := invokeTool(t, "my-list-objects",
			fmt.Sprintf(`{"bucket": %q, "prefix": "seed/"}`, bucket))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if !strings.Contains(result, helloObject) {
			t.Errorf("expected result to contain %q, got %s", helloObject, result)
		}
		if !strings.Contains(result, jsonObject) {
			t.Errorf("expected result to contain %q, got %s", jsonObject, result)
		}
	})

	t.Run("list with delimiter returns prefixes", func(t *testing.T) {
		result, status := invokeTool(t, "my-list-objects",
			fmt.Sprintf(`{"bucket": %q, "prefix": "seed/", "delimiter": "/"}`, bucket))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if !strings.Contains(result, helloObject) {
			t.Errorf("expected result to contain %q, got %s", helloObject, result)
		}
		if !strings.Contains(result, `"seed/nested/"`) {
			t.Errorf("expected result to contain prefix 'seed/nested/', got %s", result)
		}
	})

	t.Run("pagination via max_results and page_token", func(t *testing.T) {
		result, status := invokeTool(t, "my-list-objects",
			fmt.Sprintf(`{"bucket": %q, "prefix": "seed/", "max_results": 1}`, bucket))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		token := extractStringField(t, result, "nextPageToken")
		if token == "" {
			t.Fatalf("expected non-empty nextPageToken, got %s", result)
		}

		result2, status := invokeTool(t, "my-list-objects",
			fmt.Sprintf(`{"bucket": %q, "prefix": "seed/", "page_token": %q}`, bucket, token))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result2)
		}
		// Combined, the two pages should mention both seed objects.
		combined := result + result2
		if !strings.Contains(combined, helloObject) || !strings.Contains(combined, jsonObject) {
			t.Errorf("expected both %q and %q across paginated results, got page1=%s page2=%s",
				helloObject, jsonObject, result, result2)
		}
	})

	t.Run("missing bucket parameter returns agent error", func(t *testing.T) {
		result, status := invokeTool(t, "my-list-objects", `{}`)
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if !strings.Contains(result, "bucket") {
			t.Errorf("expected error mentioning 'bucket', got %s", result)
		}
	})

	t.Run("nonexistent bucket returns error", func(t *testing.T) {
		fake := "toolbox-it-does-not-exist-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
		result, _ := invokeTool(t, "my-list-objects",
			fmt.Sprintf(`{"bucket": %q}`, fake))
		if !strings.Contains(strings.ToLower(result), "error") && !strings.Contains(result, fake) {
			t.Errorf("expected error for nonexistent bucket, got %s", result)
		}
	})
}

func runCloudStorageReadObjectTest(t *testing.T, bucket string) {
	t.Run("read full object", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": %q}`, bucket, helloObject))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		decoded := decodeBase64Field(t, result, "content")
		if decoded != helloBody {
			t.Errorf("expected %q, got %q (raw %s)", helloBody, decoded, result)
		}
		if ct := extractStringField(t, result, "contentType"); ct != "text/plain" {
			t.Errorf("expected contentType text/plain, got %q", ct)
		}
	})

	t.Run("read range bytes=0-4", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": %q, "range": "bytes=0-4"}`, bucket, helloObject))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if decoded := decodeBase64Field(t, result, "content"); decoded != "hello" {
			t.Errorf("expected %q, got %q (raw %s)", "hello", decoded, result)
		}
	})

	t.Run("read suffix range bytes=-5", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": %q, "range": "bytes=-5"}`, bucket, helloObject))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if decoded := decodeBase64Field(t, result, "content"); decoded != "world" {
			t.Errorf("expected %q, got %q (raw %s)", "world", decoded, result)
		}
	})

	t.Run("read open-ended range bytes=6-", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": %q, "range": "bytes=6-"}`, bucket, helloObject))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if decoded := decodeBase64Field(t, result, "content"); decoded != "world" {
			t.Errorf("expected %q, got %q (raw %s)", "world", decoded, result)
		}
	})

	t.Run("missing object parameter returns agent error", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q}`, bucket))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if !strings.Contains(result, "object") {
			t.Errorf("expected error mentioning 'object', got %s", result)
		}
	})

	t.Run("nonexistent object returns error", func(t *testing.T) {
		result, _ := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": "does/not/exist.bin"}`, bucket))
		if !strings.Contains(strings.ToLower(result), "error") && !strings.Contains(result, "does/not/exist.bin") {
			t.Errorf("expected error for nonexistent object, got %s", result)
		}
	})

	t.Run("invalid range returns agent error", func(t *testing.T) {
		result, status := invokeTool(t, "my-read-object",
			fmt.Sprintf(`{"bucket": %q, "object": %q, "range": "garbage"}`, bucket, helloObject))
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", status, result)
		}
		if !strings.Contains(result, "range") {
			t.Errorf("expected error mentioning 'range', got %s", result)
		}
	})
}

// extractStringField pulls a top-level string field out of a JSON-encoded result
// string (the kind the tool invoke API wraps in the `result` property).
func extractStringField(t *testing.T, result, field string) string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse tool result JSON: %s (raw=%s)", err, result)
	}
	v, _ := parsed[field].(string)
	return v
}

// decodeBase64Field extracts a base64-encoded string field and returns its
// decoded UTF-8 value, failing the test if decoding fails.
func decodeBase64Field(t *testing.T, result, field string) string {
	t.Helper()
	encoded := extractStringField(t, result, field)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to base64-decode field %q: %s (encoded=%s)", field, err, encoded)
	}
	return string(decoded)
}
