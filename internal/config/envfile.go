package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/omar0ali/watchd/internal/paths"
)

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseEnvFile reads a dotenv-style file:
//
//	# comment
//	KEY=value
//	export KEY="quoted\nvalue"
//	KEY='literal $value'
//	KEY=value # trailing comment
func ParseEnvFile(path string) (map[string]string, error) {
	if err := paths.CheckNotWorldWritable(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	env := map[string]string{}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKeyRe.MatchString(key) {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, n)
		}
		val = strings.TrimSpace(val)
		switch {
		case len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"':
			uq, err := strconv.Unquote(val)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: bad quoted value: %v", path, n, err)
			}
			val = uq
		case len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'':
			val = val[1 : len(val)-1]
		default:
			if i := strings.Index(val, " #"); i >= 0 {
				val = strings.TrimSpace(val[:i])
			}
		}
		env[key] = val
	}
	return env, sc.Err()
}
