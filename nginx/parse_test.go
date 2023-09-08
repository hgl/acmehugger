package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	names, err := filepath.Glob("testdata/parse/*.in.conf")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range names {
		base := strings.TrimSuffix(src, ".in.conf")
		name := filepath.Base(base)
		tr, err := Parse(src, filepath.Dir(src))
		if err != nil {
			t.Error(err)
			continue
		}
		tmpDir := t.TempDir()
		n, err := tr.Dump(tmpDir)
		if err != nil {
			t.Error(name, err)
			continue
		}
		data, err := os.ReadFile(n)
		if err != nil {
			t.Error(err)
			continue
		}
		got := string(data)
		dir, err := filepath.Abs(src)
		if err != nil {
			t.Error(err)
			continue
		}
		dir = filepath.Dir(dir)
		dir = filepath.Join(tmpDir, dir)
		got = strings.ReplaceAll(got, dir, "")
		data, err = os.ReadFile(base + ".out.conf")
		if err != nil {
			t.Error(name, err)
			continue
		}
		want := string(data)
		if got != want {
			t.Errorf("%s: \ngot\n%#v\nwant\n%#v", name, got, want)
			return
		}
		t.Log(name, "OK")
	}
}

func TestAST(t *testing.T) {
	name := "testdata/parse/inc-main.ast.conf"
	tr, err := Parse(name, "testdata/parse")
	if err != nil {
		t.Error(err)
		return
	}
	block := tr.Config().Children[0].(*BlockDirective)
	inc := block.Children[0].(*IncludeDirective)
	d := inc.Includes[0].Children[0].(*IncludeDirective).Includes[0].Children[0].(*SimpleDirective)
	if d.ParentBlock() != block {
		t.Errorf("%s: got\n\t%+v\nexpected\n\t%v", name, d.ParentBlock(), block)
		return
	}
}

func TestParseErr(t *testing.T) {
	names, err := filepath.Glob("testdata/parse/*.err.conf")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range names {
		base := strings.TrimSuffix(src, ".err.conf")
		name := filepath.Base(base)
		_, err := Parse(src, filepath.Dir(src))
		if err == nil {
			t.Errorf("%s: didn't fail", name)
			continue
		}
	}
}
