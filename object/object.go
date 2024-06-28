package object

import (
	"strconv"
)

type Type uint8

type Object interface {
	Type() Type
	Inspect() string
}

const (
	UNKNOWN Type = iota
	INTEGER
	BOOLEAN
	NULL
	ERROR
	LAST
)

//go:generate stringer -type=Type
var _ = LAST.String() // force compile error if go generate is missing.

type Integer struct {
	Value int64
}

func (i *Integer) Inspect() string {
	return strconv.FormatInt(i.Value, 10)
}

func (i *Integer) Type() Type {
	return INTEGER
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() Type {
	return BOOLEAN
}

func (b *Boolean) Inspect() string {
	return strconv.FormatBool(b.Value)
}

type Null struct{}

func (n *Null) Type() Type      { return NULL }
func (n *Null) Inspect() string { return "<null>" }

type Error struct {
	Value string // message
}

func (e *Error) Type() Type      { return ERROR }
func (e *Error) Inspect() string { return "<err: " + e.Value + ">" }
