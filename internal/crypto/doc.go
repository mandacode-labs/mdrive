// Package crypto holds the symmetric primitives mdrive uses to
// protect secrets at rest in the database. The S3 data path is
// not encrypted by this package; S3 server-side encryption
// (SSE-S3, default) handles at-rest protection of object bodies.
package crypto
