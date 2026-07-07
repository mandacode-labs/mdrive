// Package s3 wraps the AWS S3 SDK for fs.Storage presign/verify
// operations. One Presigner instance per bucket; the parent
// vfs layer caches per superblock.
package s3