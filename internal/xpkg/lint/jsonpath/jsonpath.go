package jsonpath

import (
	"fmt"
	"strings"
)

type JSONPathSegment interface {
	String() string
}

type FieldSegment string

func (f FieldSegment) String() string {
	if strings.ContainsRune(string(f), '.') {
		return fmt.Sprintf("[%s]", string(f))
	}
	return fmt.Sprintf(".%s", string(f))
}

type IndexSegment int

func (i IndexSegment) String() string {
	return fmt.Sprintf("[%d]", i)
}

type JSONPath []JSONPathSegment

func (p JSONPath) String() string {
	b := strings.Builder{}
	for _, s := range p {
		b.WriteString(s.String())
	}
	return b.String()
}

func NewJSONPath(rawSegments ...any) JSONPath {
	path := make(JSONPath, len(rawSegments))
	for i, s := range rawSegments {
		switch val := s.(type) {
		case JSONPathSegment:
			path[i] = val
		case int:
			path[i] = IndexSegment(val)
		case string:
			path[i] = FieldSegment(val)
		default:
			path[i] = FieldSegment(fmt.Sprint(val))
		}
	}
	return path
}
