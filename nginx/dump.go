package nginx

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (tr *Tree) Dump(outdir string) (name string, err error) {
	defer func() {
		if err == nil {
			slog.Debug("config dumped", "path", name)
		}
	}()
	return tr.conf.dump(outdir)
}

func (conf *Config) dump(outdir string) (name string, err error) {
	defer conf.recover(&err)
	name = filepath.Join(outdir, conf.path)
	dir := filepath.Dir(name)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer func() {
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
	}()
	conf.out = f
	for _, d := range conf.Children {
		conf.dumpDirective(d, 0, outdir)
	}
	return name, nil
}

func (conf *Config) write(s string) {
	_, err := io.WriteString(conf.out, s)
	if err != nil {
		panic(err)
	}
}

func (conf *Config) recover(errp *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if *errp == nil {
			*errp = e.(error)
		}
	}
}

func (conf *Config) dumpDirective(dire Directive, depth int, outdir string) {
	switch d := dire.(type) {
	case *SimpleDirective:
		for i := 0; i < depth; i++ {
			conf.write("\t")
		}
		for i, arg := range d.raw {
			if i != 0 {
				conf.write(" ")
			}
			conf.write(arg)
		}
		conf.write(";\n")
		return
	case *IncludeDirective:
		for i := 0; i < depth; i++ {
			conf.write("\t")
		}
		conf.write(d.Name())
		conf.write(" ")
		target := d.Target
		if !filepath.IsAbs(d.Target) {
			target = filepath.Join(d.conf.tr.confdir, target)
		}
		target = filepath.Join(outdir, target)
		conf.write(escape(target))
		conf.write(";\n")
		for _, sub := range d.Includes {
			_, err := sub.dump(outdir)
			if err != nil {
				panic(err)
			}
		}
		return
	case *BlockDirective:
		for i := 0; i < depth; i++ {
			conf.write("\t")
		}
		for i, arg := range d.raw {
			if i != 0 {
				conf.write(" ")
			}
			conf.write(arg)
		}
		conf.write(" {")
		if len(d.Children) == 0 {
			conf.write("}\n")
		} else {
			conf.write("\n")
			for _, child := range d.Children {
				conf.dumpDirective(child, depth+1, outdir)
			}
			for i := 0; i <= depth-1; i++ {
				conf.write("\t")
			}
			conf.write("}\n")
		}
		return
	case *DeferredDirective:
		return
	default:
		panic("unknown Directive")
	}
}

func escape(v string) string {
	if v == "" {
		return `""`
	}
	var b strings.Builder
	for width, i := 0, 0; i < len(v); i += width {
		var r rune
		r, width = utf8.DecodeRune([]byte(v)[i:])
		switch r {
		case '"', '\'':
			if i == 0 {
				b.WriteRune('\\')
			}
		case ';', '{':
			b.WriteRune('\\')
		default:
			if unicode.IsSpace(r) {
				b.WriteRune('\\')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
