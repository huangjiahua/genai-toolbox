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
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/goccy/go-yaml"
	"github.com/googleapis/mcp-toolbox/internal/sources"
	"github.com/googleapis/mcp-toolbox/internal/util"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const SourceType string = "cloud-storage"

// validate interface
var _ sources.SourceConfig = Config{}

func init() {
	if !sources.Register(SourceType, newConfig) {
		panic(fmt.Sprintf("source type %q already registered", SourceType))
	}
}

func newConfig(ctx context.Context, name string, decoder *yaml.Decoder) (sources.SourceConfig, error) {
	actual := Config{Name: name}
	if err := decoder.DecodeContext(ctx, &actual); err != nil {
		return nil, err
	}
	return actual, nil
}

type Config struct {
	Name    string `yaml:"name" validate:"required"`
	Type    string `yaml:"type" validate:"required"`
	Project string `yaml:"project" validate:"required"`
}

func (r Config) SourceConfigType() string {
	return SourceType
}

func (r Config) Initialize(ctx context.Context, tracer trace.Tracer) (sources.Source, error) {
	client, err := initGCSClient(ctx, tracer, r.Name, r.Project)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	s := &Source{
		Config: r,
		Client: client,
	}
	return s, nil
}

var _ sources.Source = &Source{}

type Source struct {
	Config
	Client *storage.Client
}

func (s *Source) SourceType() string {
	return SourceType
}

func (s *Source) ToConfig() sources.SourceConfig {
	return s.Config
}

func (s *Source) StorageClient() *storage.Client {
	return s.Client
}

func (s *Source) GetProjectID() string {
	return s.Project
}

// ListObjects lists objects in a bucket with optional prefix and delimiter filtering.
// maxResults == 0 means return up to one page as returned by the GCS API. A non-empty
// pageToken resumes listing from a prior call. The returned map contains "objects"
// (raw *storage.ObjectAttrs entries as returned by the GCS client), "prefixes"
// (common prefixes when a delimiter is set), and "nextPageToken" (empty when
// there are no more results).
func (s *Source) ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxResults int, pageToken string) (map[string]any, error) {
	it := s.Client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: delimiter,
	})
	pager := iterator.NewPager(it, pageSize(maxResults), pageToken)

	var attrsPage []*storage.ObjectAttrs
	nextPageToken, err := pager.NextPage(&attrsPage)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %q: %w", bucket, err)
	}

	objects := make([]*storage.ObjectAttrs, 0, len(attrsPage))
	prefixes := make([]string, 0)
	for _, attrs := range attrsPage {
		if attrs.Prefix != "" {
			prefixes = append(prefixes, attrs.Prefix)
			continue
		}
		objects = append(objects, attrs)
	}

	return map[string]any{
		"objects":       objects,
		"prefixes":      prefixes,
		"nextPageToken": nextPageToken,
	}, nil
}

// ReadObject fetches an object's bytes and returns a map with the
// base64-encoded content, its content type, and the number of bytes read.
// offset and length follow storage.ObjectHandle.NewRangeReader semantics:
// length == -1 means "read to end of object"; a negative offset means "suffix
// from end" (in which case length must be -1).
func (s *Source) ReadObject(ctx context.Context, bucket, object string, offset, length int64) (map[string]any, error) {
	reader, err := s.Client.Bucket(bucket).Object(object).NewRangeReader(ctx, offset, length)
	if err != nil {
		return nil, fmt.Errorf("failed to open object %q in bucket %q: %w", object, bucket, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read object %q in bucket %q: %w", object, bucket, err)
	}

	return map[string]any{
		"content":     base64.StdEncoding.EncodeToString(data),
		"contentType": reader.Attrs.ContentType,
		"size":        len(data),
	}, nil
}

// pageSize returns the effective page size for pagination. The GCS API defaults
// to 1000 when a non-positive value is supplied.
func pageSize(maxResults int) int {
	if maxResults <= 0 {
		return 1000
	}
	return maxResults
}

func initGCSClient(ctx context.Context, tracer trace.Tracer, name, project string) (*storage.Client, error) {
	//nolint:all // Reassigned ctx
	ctx, span := sources.InitConnectionSpan(ctx, tracer, SourceType, name)
	defer span.End()

	userAgent, err := util.UserAgentFromContext(ctx)
	if err != nil {
		return nil, err
	}

	client, err := storage.NewClient(ctx, option.WithUserAgent(userAgent))
	if err != nil {
		return nil, fmt.Errorf("unable to create storage.NewClient for project %q: %w", project, err)
	}
	return client, nil
}
