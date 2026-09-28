package migrate

import (
	"strings"
	"unicode"
)

// optionNames are trailing CREATE TABLE options HardhatDB does not store.
// Longer names come first so CHARSET does not win over CHARACTER SET.
var optionNames = []string{
	"CHARACTER SET",
	"AUTO_INCREMENT",
	"ROW_FORMAT",
	"CHARSET",
	"COLLATE",
	"ENGINE",
	"COMMENT",
}

// RewriteCreateTable removes DEFINER and trailing table options that
// HardhatDB does not store. Column definitions, keys, and constraints stay.
func RewriteCreateTable(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	stmt = strings.TrimSuffix(stmt, ";")
	stmt = stripDefiner(stmt)
	prefix, suffix, ok := splitTableBody(stmt)
	if !ok {
		return strings.TrimSpace(stmt)
	}
	idx := strings.IndexByte(prefix, '(')
	if idx > 0 {
		prefix = tighten(prefix[:idx]) + prefix[idx:]
	}
	kept := filterTableOptions(suffix)
	if kept == "" {
		return strings.TrimSpace(prefix)
	}
	return strings.TrimSpace(prefix) + " " + kept
}

func stripDefiner(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		rest, ok := cutDefiner(s[i:])
		if ok {
			i = len(s) - len(rest)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func cutDefiner(s string) (string, bool) {
	r, ok := matchKeyword(s, "DEFINER")
	if !ok {
		return s, false
	}
	r = skipSpace(r)
	if !strings.HasPrefix(r, "=") {
		return s, false
	}
	r = skipSpace(r[1:])
	r, ok = consumeAccount(r)
	if !ok {
		return s, false
	}
	return r, true
}

func consumeAccount(s string) (string, bool) {
	if r, ok := matchKeyword(s, "CURRENT_USER"); ok {
		r = skipSpace(r)
		if strings.HasPrefix(r, "(") {
			r = skipSpace(r[1:])
			if strings.HasPrefix(r, ")") {
				return skipSpace(r[1:]), true
			}
			return s, false
		}
		return r, true
	}
	r, ok := consumeValue(s)
	if !ok {
		return s, false
	}
	r = skipSpace(r)
	if !strings.HasPrefix(r, "@") {
		return s, false
	}
	r = skipSpace(r[1:])
	return consumeValue(r)
}

// splitTableBody returns the statement through the closing paren of the
// column list, and the table options after it.
func splitTableBody(stmt string) (prefix, suffix string, ok bool) {
	in := byte(0)
	depth := 0
	for i := 0; i < len(stmt); i++ {
		c := stmt[i]
		if in != 0 {
			if c == '\\' && in != '`' && i+1 < len(stmt) {
				i++
				continue
			}
			if c == in {
				if in != '`' && i+1 < len(stmt) && stmt[i+1] == in {
					i++
					continue
				}
				in = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			in = c
		case '(':
			depth++
		case ')':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 {
				return stmt[:i+1], stmt[i+1:], true
			}
		}
	}
	return stmt, "", false
}

func filterTableOptions(suffix string) string {
	var b strings.Builder
	s := suffix
	for len(s) > 0 {
		if atBoundary(b.String()) {
			if rest, ok := cutKnownOption(s); ok {
				s = rest
				continue
			}
		}
		if s[0] == '\'' || s[0] == '"' || s[0] == '`' {
			rest, ok := consumeQuoted(s)
			if !ok {
				b.WriteString(s)
				break
			}
			b.WriteString(s[:len(s)-len(rest)])
			s = rest
			continue
		}
		b.WriteByte(s[0])
		s = s[1:]
	}
	return strings.Trim(tighten(b.String()), " \t\r\n,")
}

func atBoundary(kept string) bool {
	if kept == "" {
		return true
	}
	return !isIdentByte(kept[len(kept)-1])
}

func cutKnownOption(s string) (string, bool) {
	body := s
	if r, ok := matchKeyword(s, "DEFAULT"); ok {
		body = skipSpace(r)
	}
	for _, name := range optionNames {
		r, ok := matchKeyword(body, name)
		if !ok {
			continue
		}
		r = skipSpace(r)
		if strings.HasPrefix(r, "=") {
			r = skipSpace(r[1:])
		}
		r, ok = consumeValue(r)
		if !ok {
			return s, false
		}
		return r, true
	}
	return s, false
}

func matchKeyword(s, word string) (string, bool) {
	if len(s) < len(word) || !strings.EqualFold(s[:len(word)], word) {
		return s, false
	}
	if len(s) > len(word) && isIdentByte(s[len(word)]) {
		return s, false
	}
	return s[len(word):], true
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

func skipSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func consumeValue(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	if s[0] == '\'' || s[0] == '"' || s[0] == '`' {
		return consumeQuoted(s)
	}
	i := 0
	for i < len(s) && !unicode.IsSpace(rune(s[i])) && s[i] != ',' {
		i++
	}
	if i == 0 {
		return s, false
	}
	return s[i:], true
}

func consumeQuoted(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	q := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' && q != '`' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == q {
			if q != '`' && i+1 < len(s) && s[i+1] == q {
				i++
				continue
			}
			return s[i+1:], true
		}
	}
	return s, false
}

// tighten collapses space runs outside quotes to a single space.
func tighten(s string) string {
	var b strings.Builder
	in := byte(0)
	prevSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			b.WriteByte(c)
			if c == '\\' && in != '`' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
				continue
			}
			if c == in {
				if in != '`' && i+1 < len(s) && s[i+1] == in {
					i++
					b.WriteByte(s[i])
					continue
				}
				in = 0
			}
			prevSpace = false
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			in = c
			prevSpace = false
			b.WriteByte(c)
			continue
		}
		if unicode.IsSpace(rune(c)) {
			if prevSpace {
				continue
			}
			prevSpace = true
			b.WriteByte(' ')
			continue
		}
		prevSpace = false
		b.WriteByte(c)
	}
	return b.String()
}
