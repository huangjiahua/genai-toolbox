---
title: "cloud-storage-read-object"
type: docs
weight: 2
description: >
  A "cloud-storage-read-object" tool reads the content of a Cloud Storage object and returns it as a base64-encoded string, optionally constrained to a byte range.
---

## About

A `cloud-storage-read-object` tool fetches the bytes of a single
[Cloud Storage object][gcs-objects] and returns them base64-encoded so that
arbitrary binary content can be round-tripped through JSON safely. For large
objects, prefer the optional `range` parameter to read only the bytes you need.

This tool is intended for small-to-medium textual or binary content an LLM can
process directly. For bulk downloads of large files to the local filesystem,
use `cloud-storage-download-object` (coming in a follow-up release).

[gcs-objects]: https://cloud.google.com/storage/docs/objects

## Compatible Sources

{{< compatible-sources >}}

## Parameters

| **parameter** | **type** | **required** | **description**                                                                                                                                   |
|---------------|:--------:|:------------:|---------------------------------------------------------------------------------------------------------------------------------------------------|
| bucket        |  string  |     true     | Name of the Cloud Storage bucket containing the object.                                                                                           |
| object        |  string  |     true     | Full object name (path) within the bucket, e.g. `path/to/file.txt`.                                                                               |
| range         |  string  |    false     | Optional HTTP byte range, e.g. `bytes=0-999` (first 1000 bytes), `bytes=-500` (last 500 bytes), or `bytes=500-` (from byte 500 to end). Empty reads the full object. |

## Example

```yaml
kind: tool
name: read_object
type: cloud-storage-read-object
source: my-gcs-source
description: Use this tool to read the content of a Cloud Storage object.
```

## Reference

| **field**   | **type** | **required** | **description**                                         |
|-------------|:--------:|:------------:|---------------------------------------------------------|
| type        |  string  |     true     | Must be "cloud-storage-read-object".                    |
| source      |  string  |     true     | Name of the Cloud Storage source to read the object from. |
| description |  string  |     true     | Description of the tool that is passed to the LLM.      |
