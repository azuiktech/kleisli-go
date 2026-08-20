package adt

// Unit is a type alias for struct{}, freely interchangeable with struct{} in
// any context. Use it to make Result[Unit] and Option[Unit] readable.
type Unit = struct{}

// Void is a pre-created Unit value — use adt.OK(adt.Void) or adt.Some(adt.Void)
// instead of adt.OK(adt.Unit{}) or adt.Some(adt.Unit{}).
var Void = Unit{}
