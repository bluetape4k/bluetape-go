// Package etcdleader provides etcd v3 election-backed leader election over a
// caller-owned client.
//
// Construct electors with [New]. The zero value is unusable. New validates
// local configuration and generates an owner token without contacting etcd;
// the caller remains responsible for closing the client.
package etcdleader
