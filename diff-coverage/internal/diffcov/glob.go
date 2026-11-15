package diffcov

import (
	"path"
	"regexp"
	"strings"
)

// matcher matches slash-separated relative paths against globs. `*` and `?`
// stay within a path segment, `**` crosses segments, and a pattern without a
// slash also matches the basename at any depth.
type matcher struct {
	res []*regexp.Regexp
}

func newMatcher(globs []string) (*matcher, error) {
	m := &matcher{}
	for _, g := range globs {
		if !strings.Contains(g, "/") {
			g = "**/" + g
		}
		re, err := regexp.Compile(globToRegexp(g))
		if err != nil {
			return nil, err
		}
		m.res = append(m.res, re)
	}
	return m, nil
}

func (m *matcher) match(p string) bool {
	p = path.Clean(p)
	for _, re := range m.res {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}

func globToRegexp(g string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		switch c := g[i]; c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				i++
				if i+1 < len(g) && g[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}
