// Package s3 wraps the AWS S3 SDK for storage operations used
// by both fs (presign/verify) and drive (storage validation).
// All functions are stateless; callers own the *s3.Client.
package s3