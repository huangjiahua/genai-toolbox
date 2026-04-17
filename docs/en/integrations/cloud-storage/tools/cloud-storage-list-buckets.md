---
title: "cloud-storage-list-buckets"
type: docs
weight: 3
description: >
  A "cloud-storage-list-buckets" tool lists the buckets in the project the source is configured with, with optional prefix filtering.
---

## About

A `cloud-storage-list-buckets` tool returns the
[Cloud Storage buckets][gcs-buckets] in the project that the source is
configured with. It supports:

- `prefix` — filter results to buckets whose names begin with the given string.
- `max_results` / `page_token` — paginate through large listings.

The response is a JSON object with `buckets` (the full bucket metadata as
returned by the Cloud Storage API — fields such as `Name`, `Location`,
`StorageClass`, `Created`, `Labels`, `VersioningEnabled`, etc.) and
`nextPageToken` (empty when there are no more pages).

[gcs-buckets]: https://cloud.google.com/storage/docs/buckets

## Compatible Sources

{{< compatible-sources >}}

## Parameters

| **parameter** | **type** | **required** | **description**                                                                                                   |
|---------------|:--------:|:------------:|-------------------------------------------------------------------------------------------------------------------|
| prefix        |  string  |    false     | Filter results to buckets whose names begin with this prefix.                                                     |
| max_results   | integer  |    false     | Maximum number of buckets to return per page. A value of 0 uses the API default (1000); the maximum allowed is 1000. |
| page_token    |  string  |    false     | A previously-returned page token for retrieving the next page of results.                                         |

## Example

```yaml
kind: tool
name: list_buckets
type: cloud-storage-list-buckets
source: my-gcs-source
description: Use this tool to list the Cloud Storage buckets in the project.
```

## Reference

| **field**   | **type** | **required** | **description**                                        |
|-------------|:--------:|:------------:|--------------------------------------------------------|
| type        |  string  |     true     | Must be "cloud-storage-list-buckets".                  |
| source      |  string  |     true     | Name of the Cloud Storage source to list buckets from. |
| description |  string  |     true     | Description of the tool that is passed to the LLM.     |
