// Package codec provides small string encoders used by bluetape-go packages.
//
// Base64 and Hex wrap Go's standard library encoders. Base58 and Base62 are
// provided because they are not in the standard library and are useful for
// URL-safe identifiers, Redis keys, and compact test data. URL62 is an alias
// for the Base62 alphabet because that alphabet is already URL-safe.
package codec
