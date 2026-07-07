// Package provider defines the storage backend abstraction and
// the s3/minio implementations. One StorageProvider instance
// per app-level storage config; the secret is the plaintext
// credential (encryption belongs to the caller of the
// application, not the provider).
package provider