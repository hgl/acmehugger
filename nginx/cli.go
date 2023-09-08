package nginx

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/hgl/acmehugger"
	"github.com/hgl/acmehugger/acme"
)

func parseArgs(bin string, args []string) (conf string, nbin string, nargs []string, err error) {
	confArgIdx := -1
	for i, arg := range args {
		if arg == "" {
			continue
		}
		if arg[0] == '-' && arg[len(arg)-1] == 'c' {
			if i+1 >= len(args) {
				return "", "", nil, errors.New("the -c argument requires a configuration file")
			}
			conf = args[i+1]
			confArgIdx = i
			break
		}
	}
	if confArgIdx == -1 {
		conf = Conf
		nargs = args
	} else {
		conf, err = filepath.Abs(conf)
		if err != nil {
			return "", "", nil, err
		}
		nargs = append(nargs, args[:confArgIdx]...)
		arg := args[confArgIdx]
		if arg != "-c" {
			nargs = append(nargs, arg[:len(arg)-1])
		}
		nargs = append(nargs, args[confArgIdx+2:]...)
	}
	if bin == "" {
		nbin = "nginx"
	} else {
		nbin = bin
	}
	return conf, nbin, nargs, nil
}

func Run() error {
	if os.Getenv("ACMEHUGGER_DEBUG") != "" {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h))
	}
	conf, bin, args, err := parseArgs(os.Getenv("NGINXBIN"), os.Args[1:])
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "-h" {
		fmt.Printf(`nginxh version: %s %s/%s
Usage: nginxh [nginx option] ...

Run 'nginx -h' for more information on nginx options.
`, acmehugger.Version, runtime.GOOS, runtime.GOARCH)
		return nil
	}
	slog.Debug("nginx args parsed", "conf", conf, "bin", bin, "args", args)

	var hup = make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	var tr *Tree
	var inst *Instance
	var ap *ACMEProcessor
	render := func() error {
		var err error
		if tr == nil {
			tr, err = Parse(conf, ConfDir)
		} else {
			err = tr.Parse()
		}
		if err != nil {
			return err
		}
		ap, err = tr.PrepareACME()
		if err != nil {
			return err
		}
		if inst == nil {
			inst, err = StartInstance(tr, bin, args)
			if err != nil {
				return err
			}
			go func() {
				err := inst.Wait()
				if err != nil {
					slog.Error("nginx exited unexpectedly", "error", err)
				} else {
					slog.Error(`nginx exited unexpectedly, did you forget to specifiy -g "daemon off;"?`)
				}
				code := ExitCode(err)
				if code == 0 {
					code = 1
				}
				os.Exit(code)
			}()
		} else {
			err = inst.Reload(tr)
			if err != nil {
				return err
			}
		}

		changed := ap.Process()
	inner:
		for {
			select {
			case info := <-changed:
				var err error
				switch info.Block.name {
				case "server":
					if info.TreeChanged {
						err = inst.Reload(info.Block.Tree())
					} else {
						err = inst.Reload(nil)
					}
					if err != nil {
						slog.Error("failed to reload nginx", "error", err)
						continue
					}
				}
				err = acme.CallHooks(&acme.HookInfo{
					Server:  info.Server,
					Email:   info.Email,
					Domains: info.Domains,
				})
				if err != nil {
					continue
				}
			case <-hup:
				slog.Debug("SIGHUP received, reloading config")
				ap.Stop()
				// TODO: remove all previously generated confs
				break inner
			}
		}
		return nil
	}

	for {
		err = render()
		if err != nil {
			slog.Error("failed to reload config", "error", err)
			<-hup
		}
	}
}
