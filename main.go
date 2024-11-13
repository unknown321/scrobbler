package main

import (
	"log/slog"
	"scrobbler/daemon"
)

func main() {
	var err error

	err = daemon.Start()
	if err != nil {
		slog.Error("daemon", "error", err.Error())
	}
}
