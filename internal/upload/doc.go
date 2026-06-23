// Package upload orchestrates the presigned-URL upload flow: the
// client requests a presigned PUT URL, PUTs the bytes directly to
// S3, then notifies the server which creates the object node and
// links it into the destination path.
package upload
