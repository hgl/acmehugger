package acme

import (
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type HookInfo struct {
	Server  string
	Email   string
	Domains []string
}

func CallHooks(info *HookInfo) error {
	entries, err := os.ReadDir(HooksDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		err := runHookEntry(entry, info)
		if err != nil {
			slog.Error("failed to run hook", "name", entry.Name(), "error", err)
		}
	}
	return nil
}

func runHookEntry(entry fs.DirEntry, info *HookInfo) error {
	fsinfo, err := entry.Info()
	if err != nil {
		return err
	}
	mode := fsinfo.Mode()
	if mode&fs.ModeType != 0 || mode&0111 == 0 {
		return nil
	}
	name := filepath.Join(HooksDir, entry.Name())
	cmd := exec.Command(name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = []string{
		"ACME_SERVER=" + info.Server,
		"ACME_EMAIL=" + info.Email,
		"ACME_DOMAIN=" + strings.Join(info.Domains, " "),
	}
	return cmd.Run()
}
