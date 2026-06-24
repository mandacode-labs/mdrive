// Package s3 provides an S3/MinIO client implementation of the
// vfs.Store interface. Object bodies are encrypted at rest with
// SSE-S3 (AES256) by S3 itself; the SDK enforces this for direct
// PUTs and presigned PUTs.
package s3
