package main

import (
	"log/slog"

	"github.com/hgl/acmehugger/nginx"
)

func main() {
	err := nginx.Start()
	if err != nil {
		slog.Error(err.Error())
	}
}
