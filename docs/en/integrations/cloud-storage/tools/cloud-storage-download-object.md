---
title: "cloud-storage-download-object"
type: docs
weight: 5
description: >
  A "cloud-storage-download-object" tool streams a Cloud Storage object to a local file on the toolbox server's filesystem.
---

## About

A `cloud-storage-download-object` tool streams the full bytes of a
[Cloud Storage object][gcs-objects] directly to a file on the toolbox
server's local filesystem. Unlike `cloud-storage-read-object`, the object
bytes never land in the LLM's context, so there is no size cap and binary
objects are supported.

This tool is intended for toolbox deployments running as a **local MCP
server** — the destination path is accepted as given (after `filepath.Clean`
and `filepath.Abs` resolution) without sandboxing, the same trust model as
any CLI the user would run themselves. Do not expose this tool on a remote
multi-tenant deployment.

The response is a JSON object with the absolute `destination` path the bytes
were written to, the number of bytes written (`size`), and the object's
`contentType`.

[gcs-objects]: https://cloud.google.com/storage/docs/objects

## Compatible Sources

{{< compatible-sources >}}

## Parameters

| **parameter** | **type** | **required** | **description**                                                                                             |
|---------------|:--------:|:------------:|-------------------------------------------------------------------------------------------------------------|
| bucket        |  string  |     true     | Name of the Cloud Storage bucket containing the object.                                                     |
| object        |  string  |     true     | Full object name (path) within the bucket, e.g. `path/to/file.txt`.                                         |
| destination   |  string  |     true     | Local filesystem path where the object will be written. Relative paths resolve against the server's CWD.    |

## Example

```yaml
kind: tool
name: download_object
type: cloud-storage-download-object
source: my-gcs-source
description: Use this tool to download a Cloud Storage object to a local file.
```

## Reference

| **field**   | **type** | **required** | **description**                                             |
|-------------|:--------:|:------------:|-------------------------------------------------------------|
| type        |  string  |     true     | Must be "cloud-storage-download-object".                    |
| source      |  string  |     true     | Name of the Cloud Storage source to download the object from. |
| description |  string  |     true     | Description of the tool that is passed to the LLM.          |
