package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// table reads typed fields from a Lua table, recording type errors and which
// keys were read so unknown (usually misspelled) keys can be reported.
type table struct {
	t     *lua.LTable
	where string
	errs  *errList
	seen  map[string]bool
}

func newTable(t *lua.LTable, where string, errs *errList) *table {
	return &table{t: t, where: where, errs: errs, seen: map[string]bool{}}
}

func (t *table) raw(key string) lua.LValue {
	t.seen[key] = true
	return t.t.RawGetString(key)
}

func (t *table) typeErr(key, want string, got lua.LValue) {
	t.errs.add("%s: %s must be %s, got %s", t.where, key, want, got.Type())
}

func (t *table) str(key string) string {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return ""
	case lua.LString:
		return string(v)
	default:
		t.typeErr(key, "a string", v)
		return ""
	}
}

func (t *table) integer(key string, def int) int {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return def
	case lua.LNumber:
		if float64(v) != float64(int(v)) {
			t.errs.add("%s: %s must be a whole number", t.where, key)
		}
		return int(v)
	default:
		t.typeErr(key, "a number", v)
		return def
	}
}

func (t *table) boolean(key string, def bool) bool {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return def
	case lua.LBool:
		return bool(v)
	default:
		t.typeErr(key, "true or false", v)
		return def
	}
}

// duration accepts a number of seconds (2, 0.5) or a Go duration string
// ("300ms", "1m30s").
func (t *table) duration(key string, def time.Duration) time.Duration {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return def
	case lua.LNumber:
		if v < 0 {
			t.errs.add("%s: %s must not be negative", t.where, key)
		}
		return time.Duration(float64(v) * float64(time.Second))
	case lua.LString:
		d, err := time.ParseDuration(string(v))
		if err != nil || d < 0 {
			t.errs.add("%s: %s: invalid duration %q (try \"500ms\" or \"2s\")", t.where, key, string(v))
			return def
		}
		return d
	default:
		t.typeErr(key, "a number of seconds or a duration string", v)
		return def
	}
}

// strList accepts a single string or a list of strings.
func (t *table) strList(key string) []string {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return nil
	case lua.LString:
		return []string{string(v)}
	case *lua.LTable:
		var out []string
		for i := 1; i <= v.Len(); i++ {
			s, ok := v.RawGetInt(i).(lua.LString)
			if !ok {
				t.errs.add("%s: %s[%d] must be a string", t.where, key, i)
				continue
			}
			out = append(out, string(s))
		}
		if k, _ := v.Next(lua.LNil); v.Len() == 0 && k != lua.LNil {
			t.errs.add("%s: %s must be a list like { \"a\", \"b\" }", t.where, key)
		}
		return out
	default:
		t.typeErr(key, "a string or a list of strings", v)
		return nil
	}
}

// strMap reads { KEY = "value" }. Numbers and booleans are converted.
func (t *table) strMap(key string) map[string]string {
	out := map[string]string{}
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
	case *lua.LTable:
		v.ForEach(func(k, val lua.LValue) {
			ks, ok := k.(lua.LString)
			if !ok || !envKeyRe.MatchString(string(ks)) {
				t.errs.add("%s: %s: invalid key %s", t.where, key, k)
				return
			}
			switch val.(type) {
			case lua.LString, lua.LNumber, lua.LBool:
				out[string(ks)] = val.String()
			default:
				t.errs.add("%s: %s.%s must be a string, got %s", t.where, key, ks, val.Type())
			}
		})
	default:
		t.typeErr(key, "a table", v)
	}
	return out
}

func (t *table) fn(key string) *lua.LFunction {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return nil
	case *lua.LFunction:
		return v
	default:
		t.typeErr(key, "a function", v)
		return nil
	}
}

func (t *table) sub(key string) *table {
	switch v := t.raw(key).(type) {
	case *lua.LNilType:
		return nil
	case *lua.LTable:
		return newTable(v, fmt.Sprintf("%s.%s", t.where, key), t.errs)
	default:
		t.typeErr(key, "a table", v)
		return nil
	}
}

// checkUnknown reports keys that were never read.
func (t *table) checkUnknown() {
	var unknown []string
	t.t.ForEach(func(k, _ lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			unknown = append(unknown, k.String())
			return
		}
		if !t.seen[string(ks)] {
			unknown = append(unknown, string(ks))
		}
	})
	if len(unknown) == 0 {
		return
	}
	slices.Sort(unknown)
	known := slices.Sorted(maps.Keys(t.seen))
	t.errs.add("%s: unknown field(s) %s (known: %s)", t.where, strings.Join(unknown, ", "), strings.Join(known, ", "))
}
