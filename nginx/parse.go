package nginx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func Parse(name string, confdir string) (*Tree, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	confdir, err = filepath.Abs(confdir)
	if err != nil {
		return nil, err
	}
	tr := &Tree{
		name:    name,
		confdir: confdir,
	}
	err = tr.Parse()
	if err != nil {
		return nil, err
	}
	slog.Debug("all configs parsed", "entrypoint", name)
	return tr, err
}

func (tr *Tree) Parse() error {
	tr.includedConfs = make(map[string]*Config)
	conf, err := tr.newConfig(tr.name, nil, nil)
	tr.conf = conf
	return err
}

func (tr *Tree) newConfig(name string, parent *IncludeDirective, parentBlock *BlockDirective) (*Config, error) {
	c := tr.includedConfs[name]
	if c != nil {
		return c, nil
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	text := string(data)
	c = &Config{
		path:        name,
		parent:      parent,
		parentBlock: parentBlock,
		text:        text,
		lexer:       newLexer(text),
		tr:          tr,
	}
	children, err := c.parseDirectives()
	if err != nil {
		return nil, err
	}
	c.Children = children
	slog.Debug("config parsed", "name", name)
	return c, nil
}

func (conf *Config) parseDirectives() ([]Directive, error) {
	var ds []Directive
	for {
		tok, atEOF := conf.lexer.NextToken()
		if atEOF {
			return ds, nil
		}
		switch tok.Type {
		case tokenPunc:
			if tok.Text != ";" {
				return nil, conf.unexpected(tok)
			}
		case tokenLiteral:
			d, err := conf.parseDirective(tok, conf, conf.parentBlock)
			if err != nil {
				return nil, err
			}
			ds = append(ds, d)
		default:
			panic("unknown tokenType")
		}
	}
}

func (d *BlockDirective) parseDirectives() ([]Directive, error) {
	var ds []Directive
	for {
		tok, atEOF := d.conf.lexer.NextToken()
		if atEOF {
			return nil, io.ErrUnexpectedEOF
		}
		switch tok.Type {
		case tokenPunc:
			switch tok.Text {
			case "}":
				return ds, nil
			case ";":
			default:
				return nil, d.conf.unexpected(tok)
			}
		case tokenLiteral:
			d, err := d.conf.parseDirective(tok, d, d)
			if err != nil {
				return nil, err
			}
			ds = append(ds, d)
		default:
			panic("unknown tokenType")
		}
	}
}

func (conf *Config) parseDirective(nameTok token, parent any, parentBlock *BlockDirective) (Directive, error) {
	name := nameTok.Value
	if name == "include" {
		return conf.parseInclude(nameTok.Pos, parent, parentBlock)
	}

	var tok token
	var args []string
	raw := []string{nameTok.Text}
outer:
	for {
		var atEOF bool
		tok, atEOF = conf.lexer.NextToken()
		if atEOF {
			return nil, io.ErrUnexpectedEOF
		}
		switch tok.Type {
		case tokenPunc:
			break outer
		case tokenLiteral:
			args = append(args, tok.Value)
			raw = append(raw, tok.Text)
		default:
			panic("unknown tokenType")
		}
	}
	switch tok.Text {
	case ";":
		return &SimpleDirective{
			pos:         nameTok.Pos,
			name:        name,
			args:        args,
			raw:         raw,
			conf:        conf,
			parent:      parent,
			parentBlock: parentBlock,
		}, nil
	case "{":
		bd := &BlockDirective{
			pos:         nameTok.Pos,
			name:        name,
			args:        args,
			raw:         raw,
			conf:        conf,
			parent:      parent,
			parentBlock: parentBlock,
		}
		children, err := bd.parseDirectives()
		if err != nil {
			return nil, err
		}
		bd.Children = children
		return bd, nil
	default:
		return nil, conf.unexpected(tok)
	}
}

func (conf *Config) parseInclude(p pos, parent any, parentBlock *BlockDirective) (*IncludeDirective, error) {
	targetTok, atEOF := conf.lexer.NextToken()
	if atEOF {
		return nil, io.ErrUnexpectedEOF
	}
	if targetTok.Type != tokenLiteral {
		return nil, conf.unexpected(targetTok)
	}

	tok, atEOF := conf.lexer.NextToken()
	if atEOF {
		return nil, io.ErrUnexpectedEOF
	}
	if tok.Type != tokenPunc || tok.Text != ";" {
		return nil, conf.unexpected(tok)
	}
	d := &IncludeDirective{
		pos:         p,
		Target:      targetTok.Value,
		conf:        conf,
		parent:      parent,
		parentBlock: parentBlock,
	}
	target := d.Target
	if !filepath.IsAbs(d.Target) {
		target = filepath.Join(conf.tr.confdir, target)
	}
	var names []string
	if strings.Contains(filepath.Base(target), "*") {
		var err error
		names, err = filepath.Glob(target)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return d, nil
		}
	} else {
		names = []string{target}
	}
	d.Includes = make([]*Config, len(names))
	for i, name := range names {
		c, err := conf.tr.newConfig(name, d, parentBlock)
		if err != nil {
			return nil, err
		}
		d.Includes[i] = c
		conf.tr.includedConfs[name] = c
	}
	return d, nil
}

func (conf *Config) unexpected(tok token) (err error) {
	return fmt.Errorf("unexpected %q in %s:%s", tok.Text, conf.path, tok.Pos.loc(conf.text))
}
