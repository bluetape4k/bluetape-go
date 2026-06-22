// Package core provides small shared helpers used by bluetape-go packages.
//
// The package intentionally stays narrow. It contains helpers that make service
// code clearer without hiding Go's standard library:
//
//   - validation helpers return errors instead of panicking;
//   - pointer helpers remove repetitive temporary variables;
//   - zero/default helpers make generic fallback code explicit;
//   - string and number helpers cover small gaps such as UTF-8 byte truncation
//     and prefixed hexadecimal checks.
//   - wildcard and XXH64 helpers cover repeated cache/key/filter use cases
//     without hiding filesystem, OS, or cryptographic boundaries.
//
// Prefer the standard library when it already expresses the operation clearly.
package core
