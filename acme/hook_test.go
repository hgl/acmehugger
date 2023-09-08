package acme

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hgl/acmehugger/internal/util"
)

func TestHook(t *testing.T) {
	HooksDir = t.TempDir()
	name := filepath.Join(HooksDir, "a.sh")
	content := fmt.Sprintf(`#!/bin/sh
{
	echo "$ACME_SERVER"
	echo "$ACME_EMAIL"
	echo "$ACME_DOMAIN"
} > "%s/env"
`, HooksDir)
	err := os.WriteFile(name, []byte(content), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = CallHooks(&HookInfo{
		Server:  "example",
		Email:   "foo@bar",
		Domains: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := util.ReadText(filepath.Join(HooksDir, "env"))
	if err != nil {
		t.Fatal(err)
	}
	want := `example
foo@bar
a b
`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
