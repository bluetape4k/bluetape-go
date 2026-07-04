// Package rules provides small deterministic rule-engine primitives.
//
// The package is intentionally dependency-free. It owns facts, rule contracts,
// deterministic rule-set ordering, and a sequential engine while leaving
// expression languages, YAML readers, composite groups, and forward chaining to
// higher-level packages.
package rules
