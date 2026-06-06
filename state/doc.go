// Package state provides small finite state machine primitives.
//
// A Machine owns one current state, applies explicit event transitions, and
// lets guards veto transitions with ordinary Go errors. Public methods are safe
// for concurrent callers.
package state
