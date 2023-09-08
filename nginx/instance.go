package nginx

import (
	"log/slog"
	"os"
	"os/exec"
	"syscall"
)

type Instance exec.Cmd

func StartInstance(tr *Tree, bin string, args []string) (*Instance, error) {
	name, err := tr.Dump(ConfOutDir)
	if err != nil {
		return nil, err
	}

	args = append([]string{}, args...)
	args = append(args, "-c", name)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	slog.Debug("nginx started", "pid", cmd.Process.Pid, "bin", bin, "args", args)
	return (*Instance)(cmd), nil
}

func (inst *Instance) Reload(tr *Tree) error {
	// TODO: throttle reloading and generate config in different folder and symlink to assure being atomic
	if tr != nil {
		_, err := tr.Dump(ConfOutDir)
		if err != nil {
			return err
		}
	}
	err := (*exec.Cmd)(inst).Process.Signal(syscall.SIGHUP)
	if err != nil {
		return err
	}
	slog.Debug("SIGHUP sent to nginx")
	return nil
}

func (inst *Instance) Wait() error {
	return (*exec.Cmd)(inst).Wait()
}

func ExitCode(err error) int {
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	if err != nil {
		return 1
	}
	return 0
}
