package nginx

import "testing"

type lexTest struct {
	name   string
	input  string
	tokens []token
}

func mkTokP(text string, p int) token {
	switch text {
	case ";", "{", "}":
		return token{
			Text: text,
			Type: tokenPunc,
			Pos:  pos(p),
		}
	default:
		return token{
			Text:  text,
			Value: text,
			Type:  tokenLiteral,
			Pos:   pos(p),
		}
	}
}

func mkTok(text string, val ...string) token {
	switch text {
	case ";", "{", "}":
		return token{
			Text: text,
			Type: tokenPunc,
		}
	default:
		return token{
			Text:  text,
			Value: val[0],
			Type:  tokenLiteral,
		}
	}
}

func lex(t lexTest) (toks []token) {
	l := newLexer(t.input)
	for {
		tok, atEOF := l.NextToken()
		if atEOF {
			return
		}
		toks = append(toks, tok)
	}
}

func equal(i1, i2 []token, checkPos bool) bool {
	if len(i1) != len(i2) {
		return false
	}
	for k := range i1 {
		if i1[k].Type != i2[k].Type {
			return false
		}
		if i1[k].Text != i2[k].Text {
			return false
		}
		if i1[k].Value != i2[k].Value {
			return false
		}
		if checkPos && i1[k].Pos != i2[k].Pos {
			return false
		}
	}
	return true
}

var lexTests = []lexTest{
	{"empty", "", []token{}},
	{"spaces", " \t\n", []token{}},
	{"ident", "foo", []token{mkTok("foo", "foo")}},
	{"string", `"foo"`, []token{mkTok(`"foo"`, "foo")}},
	{"escape", `a\ b`, []token{mkTok(`a\ b`, "a b")}},
	{"str escape", `"\t"`, []token{mkTok(`"\t"`, `\t`)}},
	{"regex", `^\.php$`, []token{mkTok(`^\.php$`, `^\.php$`)}},
	{"comment", "#a", []token{}},
	{"space comment", " #a", []token{}},
	{"comment ident", "#b\na", []token{mkTok("a", "a")}},
	{"ident comment", "a#b", []token{mkTok("a", "a")}},
	{"args", `a "b"`, []token{mkTok("a", "a"), mkTok(`"b"`, "b")}},
	{";", `a b;`, []token{mkTok("a", "a"), mkTok("b", "b"), mkTok(";")}},
	{"block", `a{bc;}x`, []token{mkTok("a", "a"), mkTok("{"), mkTok("bc", "bc"), mkTok(";"), mkTok("}"), mkTok("x", "x")}},
	{"multiline", "  a \n b", []token{mkTok("a", "a"), mkTok("b", "b")}},
}

func TestLex(t *testing.T) {
	for _, test := range lexTests {
		toks := lex(test)
		if !equal(toks, test.tokens, false) {
			t.Errorf("%s: got\n\t%#v\nwant\n\t%#v", test.name, toks, test.tokens)
			return
		}
		t.Log(test.name, "OK")
	}
}

var lexPosTests = []lexTest{
	{"ident", "foo", []token{mkTokP("foo", 0)}},
	{"comment ident", "#b\na", []token{mkTokP("a", 3)}},
	{"args", "a b", []token{
		mkTokP("a", 0),
		mkTokP("b", 2),
	}},
}

func TestPos(t *testing.T) {
	for _, test := range lexPosTests {
		toks := lex(test)
		if !equal(toks, test.tokens, true) {
			t.Errorf("%s: got\n\t%+v\nwant\n\t%v", test.name, toks, test.tokens)
			return
		}
		t.Log(test.name, "OK")
	}
}
