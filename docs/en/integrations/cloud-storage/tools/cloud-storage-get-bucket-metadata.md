---
title: "cloud-storage-get-bucket-metadata"
type: docs
weight: 6
description: >
  A "cloud-storage-get-bucket-metadata" tool returns metadata for a single Cloud Storage bucket.
---

## About

A `cloud-storage-get-bucket-metadata` tool fetches the attributes of a single
[Cloud Storage bucket][gcs-buckets] — its name, location, storage class,
creation and update timestamps, versioning status, lifecycle rules, labels,
retention policy, and so on. Use this to reason about bucket configuration
without listing any of its objects.

[gcs-buckets]: https://cloud.google.com/storage/docs/buckets

## Compatible Sources

{{< compatible-sources >}}

## Parameters

| **parameter** | **type** | **required** | **description**                                            |
|---------------|:--------:|:------------:|------------------------------------------------------------|
| bucket        |  string  |     true     | Name of the Cloud Storage bucket to fetch metadata for.    |

## Example

```yaml
kind: tool
name: get_bucket_metadata
type: cloud-storage-get-bucket-metadata
source: my-gcs-source
description: Use this tool to fetch the metadata of a Cloud Storage bucket.
```

## Reference

| **field**   | **type** | **required** | **description**                                                 |
|-------------|:--------:|:------------:|-----------------------------------------------------------------|
| type        |  string  |     true     | Must be "cloud-storage-get-bucket-metadata".                    |
| source      |  string  |     true     | Name of the Cloud Storage source to fetch bucket metadata from. |
| description |  string  |     true     | Description of the tool that is passed to the LLM.              |
