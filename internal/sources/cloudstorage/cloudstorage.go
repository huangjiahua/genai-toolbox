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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"cloud.google.com/go/storage"
	"github.com/goccy/go-yaml"
	"github.com/googleapis/mcp-toolbox/internal/sources"
	"github.com/googleapis/mcp-toolbox/internal/tools/cloudstorage/cloudstoragecommon"
	"github.com/googleapis/mcp-toolbox/internal/util"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const SourceType string = "cloud-storage"

// defaultMaxReadBytes caps the payload ReadObject will return per call,
// protecting the server from OOM and keeping LLM contexts manageable. Objects
// or ranges exceeding this are rejected with ErrReadSizeLimitExceeded.
const defaultMaxReadBytes int64 = 8 << 20 // 8 MiB

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

// ReadObject fetches an object's bytes and returns a map with the UTF-8
// content, its content type, and the number of bytes read. offset and length
// follow storage.ObjectHandle.NewRangeReader semantics: length == -1 means
// "read to end of object"; a negative offset means "suffix from end" (in
// which case length must be -1). Reads larger than defaultMaxReadBytes are
// rejected with cloudstoragecommon.ErrReadSizeLimitExceeded so the caller can
// narrow the range. Objects whose bytes are not valid UTF-8 are rejected
// with cloudstoragecommon.ErrBinaryContent.
//
// TODO: MCP tool results only carry text today, so we gate this tool on
// utf8.Valid. When the toolbox supports non-text MCP content (embedded
// resources, images, blobs), expand this to detect content type and return
// binary payloads natively.
func (s *Source) ReadObject(ctx context.Context, bucket, object string, offset, length int64) (map[string]any, error) {
	reader, err := s.Client.Bucket(bucket).Object(object).NewRangeReader(ctx, offset, length)
	if err != nil {
		return nil, fmt.Errorf("failed to open object %q in bucket %q: %w", object, bucket, err)
	}
	defer reader.Close()

	if remain := reader.Remain(); remain > defaultMaxReadBytes {
		return nil, fmt.Errorf("object %q: %d bytes exceeds %d byte limit: %w",
			object, remain, defaultMaxReadBytes,
			cloudstoragecommon.ErrReadSizeLimitExceeded)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read object %q in bucket %q: %w", object, bucket, err)
	}

	if !utf8.Valid(data) {
		return nil, fmt.Errorf("object %q in bucket %q: %w", object, bucket,
			cloudstoragecommon.ErrBinaryContent)
	}

	return map[string]any{
		"content":     string(data),
		"contentType": reader.Attrs.ContentType,
		"size":        len(data),
	}, nil
}

// ListBuckets lists buckets in the source's configured project. maxResults
// == 0 uses the GCS default page size (1000). A non-empty pageToken resumes
// listing from a prior call. prefix filters buckets whose name starts with
// it. The returned map contains "buckets" (raw *storage.BucketAttrs entries)
// and "nextPageToken" (empty when there are no more results).
func (s *Source) ListBuckets(ctx context.Context, prefix string, maxResults int, pageToken string) (map[string]any, error) {
	it := s.Client.Buckets(ctx, s.Project)
	it.Prefix = prefix
	pager := iterator.NewPager(it, pageSize(maxResults), pageToken)

	var attrsPage []*storage.BucketAttrs
	nextPageToken, err := pager.NextPage(&attrsPage)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets in project %q: %w", s.Project, err)
	}

	return map[string]any{
		"buckets":       attrsPage,
		"nextPageToken": nextPageToken,
	}, nil
}

// GetObjectMetadata fetches an object's attributes without downloading its
// bytes.
func (s *Source) GetObjectMetadata(ctx context.Context, bucket, object string) (*storage.ObjectAttrs, error) {
	attrs, err := s.Client.Bucket(bucket).Object(object).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attrs for object %q in bucket %q: %w", object, bucket, err)
	}
	return attrs, nil
}

// GetBucketMetadata fetches a bucket's attributes (location, storage class,
// labels, versioning, lifecycle, etc.).
func (s *Source) GetBucketMetadata(ctx context.Context, bucket string) (*storage.BucketAttrs, error) {
	attrs, err := s.Client.Bucket(bucket).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attrs for bucket %q: %w", bucket, err)
	}
	return attrs, nil
}

// DownloadObject streams an object's bytes directly to dest on the local
// filesystem where toolbox is running. dest is resolved with filepath.Abs +
// filepath.Clean and its parent directory is created with MkdirAll. The
// returned map contains the absolute destination path, bytes written, and
// the object's content type.
//
// This tool writes to the toolbox server's filesystem, so it is intended for
// local MCP-server deployments where the user trusts tool-driven writes in
// the same way they would a CLI they ran themselves.
func (s *Source) DownloadObject(ctx context.Context, bucket, object, dest string) (map[string]any, error) {
	absDest, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve destination %q: %w", dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory for %q: %w", absDest, err)
	}

	reader, err := s.Client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open object %q in bucket %q: %w", object, bucket, err)
	}
	defer reader.Close()

	f, err := os.Create(absDest)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file %q: %w", absDest, err)
	}
	defer f.Close()

	n, err := io.Copy(f, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write object %q to %q: %w", object, absDest, err)
	}

	return map[string]any{
		"destination": absDest,
		"size":        n,
		"contentType": reader.Attrs.ContentType,
	}, nil
}

// pageSize returns the effective page size for pagination. The GCS API caps
// results at 1000 per page; we enforce the same cap here so callers don't
// pre-allocate larger buffers and so the contract matches the tool's
// 'max_results' documentation.
func pageSize(maxResults int) int {
	const gcsMaxPage = 1000
	if maxResults <= 0 || maxResults > gcsMaxPage {
		return gcsMaxPage
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
