// Package unit provides the Unit type — the zero-information type used when
// a Result or Option carries only success/failure or presence/absence, not a
// meaningful value. Equivalent to Rust's () or Haskell's Unit.
package unit

// Unit is a type alias for struct{}, freely interchangeable with struct{} in
// any context. Use it to make Result[Unit] and Option[Unit] readable.
type Unit = struct{}
