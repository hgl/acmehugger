package nginx

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestArgs(t *testing.T) {
	conf, bin, args, err := parseArgs("", []string{"-c", "foo", "bar"})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs("foo")
	if err != nil {
		t.Fatal(err)
	}
	if conf != want {
		t.Fatalf("conf got %s, want %s", conf, want)
	}
	if want := "nginx"; bin != want {
		t.Fatalf("bin got %#v, want %#v", bin, want)
	}
	if want := []string{"bar"}; !slices.Equal(args, want) {
		t.Fatalf("args got %#v, want %#v", args, want)
	}

	conf, bin, args, err = parseArgs("x", []string{"-xc", "foo"})
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.Abs("foo")
	if err != nil {
		t.Fatal(err)
	}
	if conf != want {
		t.Fatalf("conf got %s, want %s", conf, want)
	}
	if want := "x"; bin != want {
		t.Fatalf("bin got %#v, want %#v", bin, want)
	}
	if want := []string{"-x"}; !slices.Equal(args, want) {
		t.Fatalf("args got %#v, want %#v", args, want)
	}

	conf, bin, args, err = parseArgs("x", []string{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	if want := Conf; conf != want {
		t.Fatalf("conf got %s, want %s", conf, want)
	}
	if want := "x"; bin != want {
		t.Fatalf("bin got %#v, want %#v", bin, want)
	}
	if want := []string{"foo"}; !slices.Equal(args, want) {
		t.Fatalf("args got %#v, want %#v", args, want)
	}
}
