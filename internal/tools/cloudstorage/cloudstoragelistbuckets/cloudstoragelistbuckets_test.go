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

package cloudstoragelistbuckets_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/googleapis/mcp-toolbox/internal/server"
	"github.com/googleapis/mcp-toolbox/internal/testutils"
	"github.com/googleapis/mcp-toolbox/internal/tools/cloudstorage/cloudstoragelistbuckets"
)

func TestParseFromYamlCloudStorageListBuckets(t *testing.T) {
	ctx, err := testutils.ContextWithNewLogger()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	tcs := []struct {
		desc string
		in   string
		want server.ToolConfigs
	}{
		{
			desc: "basic example",
			in: `
			kind: tool
			name: list_buckets_tool
			type: cloud-storage-list-buckets
			source: my-gcs
			description: List Cloud Storage buckets in the project
			`,
			want: server.ToolConfigs{
				"list_buckets_tool": cloudstoragelistbuckets.Config{
					Name:         "list_buckets_tool",
					Type:         "cloud-storage-list-buckets",
					Source:       "my-gcs",
					Description:  "List Cloud Storage buckets in the project",
					AuthRequired: []string{},
				},
			},
		},
		{
			desc: "with auth requirements",
			in: `
			kind: tool
			name: secure_list_buckets
			type: cloud-storage-list-buckets
			source: prod-gcs
			description: List buckets with authentication
			authRequired:
				- google-auth-service
				- api-key-service
			`,
			want: server.ToolConfigs{
				"secure_list_buckets": cloudstoragelistbuckets.Config{
					Name:         "secure_list_buckets",
					Type:         "cloud-storage-list-buckets",
					Source:       "prod-gcs",
					Description:  "List buckets with authentication",
					AuthRequired: []string{"google-auth-service", "api-key-service"},
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			_, _, _, got, _, _, err := server.UnmarshalResourceConfig(ctx, testutils.FormatYaml(tc.in))
			if err != nil {
				t.Fatalf("unable to unmarshal: %s", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("incorrect parse: diff %v", diff)
			}
		})
	}
}
