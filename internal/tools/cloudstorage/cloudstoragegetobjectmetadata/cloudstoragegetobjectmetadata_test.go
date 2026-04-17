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

package cloudstoragegetobjectmetadata_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/googleapis/mcp-toolbox/internal/server"
	"github.com/googleapis/mcp-toolbox/internal/testutils"
	"github.com/googleapis/mcp-toolbox/internal/tools/cloudstorage/cloudstoragegetobjectmetadata"
)

func TestParseFromYamlCloudStorageGetObjectMetadata(t *testing.T) {
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
			name: get_object_metadata_tool
			type: cloud-storage-get-object-metadata
			source: my-gcs
			description: Get metadata for a Cloud Storage object
			`,
			want: server.ToolConfigs{
				"get_object_metadata_tool": cloudstoragegetobjectmetadata.Config{
					Name:         "get_object_metadata_tool",
					Type:         "cloud-storage-get-object-metadata",
					Source:       "my-gcs",
					Description:  "Get metadata for a Cloud Storage object",
					AuthRequired: []string{},
				},
			},
		},
		{
			desc: "with auth requirements",
			in: `
			kind: tool
			name: secure_get_object_metadata
			type: cloud-storage-get-object-metadata
			source: prod-gcs
			description: Get object metadata with authentication
			authRequired:
				- google-auth-service
				- api-key-service
			`,
			want: server.ToolConfigs{
				"secure_get_object_metadata": cloudstoragegetobjectmetadata.Config{
					Name:         "secure_get_object_metadata",
					Type:         "cloud-storage-get-object-metadata",
					Source:       "prod-gcs",
					Description:  "Get object metadata with authentication",
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
