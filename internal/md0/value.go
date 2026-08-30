package md0

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type ValueKind uint8

const (
	NullKind ValueKind = iota
	NumberKind
	StringKind
	BoolKind
	ListKind
	ObjectKind
)

type Value struct {
	Kind ValueKind
	Num  float64
	Str  string
	Bool bool
	List []Value
	Obj  map[string]Value
}

func Null() Value            { return Value{Kind: NullKind} }
func Number(v float64) Value { return Value{Kind: NumberKind, Num: v} }
func String(v string) Value  { return Value{Kind: StringKind, Str: v} }
func Boolean(v bool) Value   { return Value{Kind: BoolKind, Bool: v} }
func List(v []Value) Value   { return Value{Kind: ListKind, List: v} }
func Object(v map[string]Value) Value {
	return Value{Kind: ObjectKind, Obj: v}
}

func (v Value) String() string {
	switch v.Kind {
	case NullKind:
		return "null"
	case NumberKind:
		return strconv.FormatFloat(v.Num, 'f', -1, 64)
	case StringKind:
		return v.Str
	case BoolKind:
		return strconv.FormatBool(v.Bool)
	case ListKind:
		parts := make([]string, len(v.List))
		for i, item := range v.List {
			parts[i] = item.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ObjectKind:
		keys := make([]string, 0, len(v.Obj))
		for key := range v.Obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, key := range keys {
			parts[i] = strconv.Quote(key) + ": " + v.Obj[key].String()
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return "<invalid>"
	}
}

func (v Value) AsNumber() (float64, error) {
	if v.Kind != NumberKind {
		return 0, fmt.Errorf("expected number, got %s", v.TypeName())
	}
	return v.Num, nil
}

func (v Value) AsBool() (bool, error) {
	if v.Kind != BoolKind {
		return false, fmt.Errorf("expected boolean, got %s", v.TypeName())
	}
	return v.Bool, nil
}

func (v Value) AsList() ([]Value, error) {
	if v.Kind != ListKind {
		return nil, fmt.Errorf("expected list, got %s", v.TypeName())
	}
	return v.List, nil
}

func (v Value) AsObject() (map[string]Value, error) {
	if v.Kind != ObjectKind {
		return nil, fmt.Errorf("expected object, got %s", v.TypeName())
	}
	return v.Obj, nil
}

func (v Value) TypeName() string {
	switch v.Kind {
	case NullKind:
		return "null"
	case NumberKind:
		return "number"
	case StringKind:
		return "string"
	case BoolKind:
		return "boolean"
	case ListKind:
		return "list"
	case ObjectKind:
		return "object"
	default:
		return "invalid"
	}
}

func ValuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case NullKind:
		return true
	case NumberKind:
		return math.Abs(a.Num-b.Num) < 1e-12
	case StringKind:
		return a.Str == b.Str
	case BoolKind:
		return a.Bool == b.Bool
	case ListKind:
		if len(a.List) != len(b.List) {
			return false
		}
		for i := range a.List {
			if !ValuesEqual(a.List[i], b.List[i]) {
				return false
			}
		}
		return true
	case ObjectKind:
		if len(a.Obj) != len(b.Obj) {
			return false
		}
		for key, value := range a.Obj {
			other, ok := b.Obj[key]
			if !ok || !ValuesEqual(value, other) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
