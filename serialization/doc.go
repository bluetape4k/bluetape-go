// Package serialization defines small binary serializer contracts.
//
// The package intentionally avoids unsafe object deserialization. The default
// JSON serializer uses Go's encoding/json, and versioned envelopes make format
// and version decisions explicit before compression or storage layers build on
// top of this package.
package serialization
