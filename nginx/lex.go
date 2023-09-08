package nginx

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type pos int

func (p pos) position() pos {
	return p
}

func (p pos) loc(text string) string {
	// For dynamically generated directives
	if p == -1 {
		return "0:0"
	}
	text = text[:int(p)+1]
	column := strings.LastIndex(text, "\n")
	if column == -1 {
		column = int(p) + 1 // On first line.
	} else {
		column = int(p) - column // After the newline.
	}
	line := 1 + strings.Count(text, "\n")
	return fmt.Sprintf("%d:%d", line, column)
}

type token struct {
	Type  tokenType
	Text  string
	Value string
	Pos   pos
}

func (tok token) Val() string {
	if tok.Type != tokenLiteral {
		return ""
	}

	text := tok.Text
	switch text[0] {
	case '"', '\'':
		text = text[1 : len(text)-1]

	}
	return strings.ReplaceAll(text, `\`, "")
}

type tokenType int8

const (
	tokenPunc tokenType = iota
	tokenLiteral
)

type lexer struct {
	input string
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input, 0}
}

func (l *lexer) NextToken() (tok token, atEOF bool) {
	start := l.pos
	var r rune
	// Skip initial white spaces
loop:
	for width := 0; start < len(l.input); start += width {
		input := l.input[start:]
		r, width = utf8.DecodeRuneInString(input)
		switch {
		case r == '#':
			n := strings.IndexRune(input, '\n')
			if n == -1 {
				atEOF = true
				return
			}
			start += n
		case !unicode.IsSpace(r):
			break loop
		}
	}

	esc := false
	quote := rune(-1)
	var txt strings.Builder
	var val strings.Builder
	for width, i := 0, start; i < len(l.input); i += width {
		r, width = utf8.DecodeRuneInString(l.input[i:])
		if esc {
			esc = false
			txt.WriteRune(r)
			if quote != -1 {
				if r != quote {
					val.WriteRune('\\')
				}
			} else {
				switch r {
				case ';', '{', '}', ' ':
				default:
					val.WriteRune('\\')
				}
			}
			val.WriteRune(r)
			continue
		}
		if r == '\\' {
			esc = true
			txt.WriteRune(r)
			continue
		}
		if quote != -1 {
			if r == quote {
				txt.WriteRune(r)
				l.pos = i + width
				tok = token{
					Text:  txt.String(),
					Value: val.String(),
					Type:  tokenLiteral,
					Pos:   pos(start),
				}
				return
			}
			txt.WriteRune(r)
			val.WriteRune(r)
			continue
		}
		switch r {
		case '"', '\'':
			txt.WriteRune(r)
			if i == start {
				quote = r
			}
		case ';', '{', '}':
			if i == start {
				l.pos = i + width
				tok = token{
					Text: string(r),
					Type: tokenPunc,
					Pos:  pos(start),
				}
				return
			}
			l.pos = i
			tok = token{
				Text:  txt.String(),
				Value: val.String(),
				Type:  tokenLiteral,
				Pos:   pos(start),
			}
			return
		case '#':
			l.pos = i
			tok = token{
				Text:  txt.String(),
				Value: val.String(),
				Type:  tokenLiteral,
				Pos:   pos(start),
			}
			return
		default:
			if unicode.IsSpace(r) {
				l.pos = i + width
				tok = token{
					Text:  txt.String(),
					Value: val.String(),
					Type:  tokenLiteral,
					Pos:   pos(start),
				}
				return
			}
			txt.WriteRune(r)
			val.WriteRune(r)
		}
	}
	if start == len(l.input) {
		atEOF = true
		return
	}
	l.pos = len(l.input)
	tok = token{
		Text:  txt.String(),
		Value: val.String(),
		Type:  tokenLiteral,
		Pos:   pos(start),
	}
	return
}
