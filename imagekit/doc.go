// Package imagekit provides bounded pure-Go helpers for resizing one image and
// encoding it as JPEG or PNG.
//
// Supported input formats are JPEG, PNG, and GIF as reported by the standard
// library image decoders. Supported output formats are JPEG and PNG.
//
// The package checks context cancellation before the bounded read, after the
// bounded read, before decode, before resize, and before encode. It cannot
// preempt a blocked caller-owned io.Reader or io.Writer, nor a standard-library
// codec call that is already executing; callers that need hard I/O deadlines
// should enforce them at the I/O boundary.
//
// Transform returns encoded bytes for convenience. TransformTo writes directly
// to a caller-owned writer when partial output is acceptable. For final HTTP
// responses or storage objects, callers should use Transform or stage
// TransformTo output in a temporary buffer/object before publishing.
package imagekit
