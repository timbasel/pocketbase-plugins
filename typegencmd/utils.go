package typegencmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

type generator struct {
	strings.Builder
	indent int
}

func (gen *generator) WriteFile(filepath string) error {
	return os.WriteFile(filepath, []byte(gen.String()), 0644)
}

func (gen *generator) write(format string, a ...any) {
	gen.WriteString(strings.Repeat("  ", gen.indent) + fmt.Sprintf(format, a...))
}

func (gen *generator) writeIndent(format string, a ...any) {
	gen.indent++
	gen.write(format, a...)
	gen.indent--
}

func (gen *generator) writeNoIndent(format string, a ...any) {
	gen.WriteString(fmt.Sprintf(format, a...))
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func quote(s string) string {
	return `"` + s + `"`
}

func quoteAll(strings []string) []string {
	for i := range strings {
		strings[i] = quote(strings[i])
	}
	return strings
}

var goKeywords = []string{"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"}
var replacements = map[string]string{
	"Id": "ID",
}

func toCamelCase(s string, capitalize bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	n := strings.Builder{}
	n.Grow(len(s))
	capNext := capitalize
	prevIsCap := false
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		} else if prevIsCap && vIsCap {
			v += 'a'
			v -= 'A'
		}
		prevIsCap = vIsCap

		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	name := n.String()
	for old, new := range replacements {
		name = strings.ReplaceAll(name, old, new)
	}
	if slices.Contains(goKeywords, name) {
		return name + "_"
	}
	return name
}

func last[E any](s []E) E {
	return s[len(s)-1]
}

func filterSystemCollections(collections []*core.Collection) []*core.Collection {
	return slices.DeleteFunc(collections, func(collection *core.Collection) bool {
		return strings.HasPrefix(collection.Name, "_")
	})
}
