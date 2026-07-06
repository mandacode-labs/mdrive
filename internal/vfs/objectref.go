package vfs

// ObjectRef is the public input for WriteObject. The caller
// (typically a handler after a successful S3 upload) provides
// the bucket, key, mime type, and optional checksum. vfs
// converts this into its internal content.ObjectContent format
// and stores it inline in the node's data field.
type ObjectRef struct {
	Bucket   string
	Key      string
	Mime     string
	Checksum string
}
