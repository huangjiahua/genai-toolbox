---
title: "cloud-storage-get-object-metadata"
type: docs
weight: 4
description: >
  A "cloud-storage-get-object-metadata" tool returns metadata for a single Cloud Storage object without downloading its bytes.
---

## About

A `cloud-storage-get-object-metadata` tool fetches the attributes of a single
[Cloud Storage object][gcs-objects] — its name, size, content type, creation
and update timestamps, storage class, checksums, and any custom metadata —
without reading the object's bytes. Use this when you need to decide what to
do with an object (e.g., check its size before reading it) or to inspect
custom metadata the producer attached to it.

[gcs-objects]: https://cloud.google.com/storage/docs/objects

## Compatible Sources

{{< compatible-sources >}}

## Parameters

| **parameter** | **type** | **required** | **description**                                                     |
|---------------|:--------:|:------------:|---------------------------------------------------------------------|
| bucket        |  string  |     true     | Name of the Cloud Storage bucket containing the object.             |
| object        |  string  |     true     | Full object name (path) within the bucket, e.g. `path/to/file.txt`. |

## Example

```yaml
kind: tool
name: get_object_metadata
type: cloud-storage-get-object-metadata
source: my-gcs-source
description: Use this tool to fetch the metadata of a Cloud Storage object.
```

## Reference

| **field**   | **type** | **required** | **description**                                                |
|-------------|:--------:|:------------:|----------------------------------------------------------------|
| type        |  string  |     true     | Must be "cloud-storage-get-object-metadata".                   |
| source      |  string  |     true     | Name of the Cloud Storage source to fetch object metadata from. |
| description |  string  |     true     | Description of the tool that is passed to the LLM.             |
