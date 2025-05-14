package source

// PositionInfo provides an interface
// for the position information of a value.
// This is primarily useful for attaching position information to errors
// for values extracted from intermediary source document nodes with position/offset
// information.
type PositionInfo interface {
	GetLine() int
	GetColumn() int
}
