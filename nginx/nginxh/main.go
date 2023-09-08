package main

import (
	"log/slog"

	"github.com/hgl/acmehugger/nginx"
)

func main() {
	err := nginx.Run()
	if err != nil {
		slog.Error(err.Error())
	}
}
