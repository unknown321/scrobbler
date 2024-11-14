package daemon

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"scrobbler/audioplayer"
	"scrobbler/audioscrobbler"
	"scrobbler/device"
	"scrobbler/logreader"
	"scrobbler/parser"
	"scrobbler/playerevents"
	"scrobbler/resolver"
	"scrobbler/server"
	"strings"
)

var name = "scrobbler"

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) > 8 {
				return setting.Value[0:7]
			}
		}
	}
	return ""
}()

var SystemLogFile = "/dev/log/main"
var ListenPercent = 50

func SetupLog() {
	level := slog.LevelInfo
	ll := os.Getenv("LOGLEVEL")
	if strings.ToLower(ll) == "debug" {
		level = slog.LevelDebug
	}

	if _, err := os.Open("/tmp/scrd"); err == nil {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       level,
		ReplaceAttr: nil,
	}

	var th slog.Handler
	th = slog.NewTextHandler(os.Stdout, opts)

	nl := slog.New(th)
	slog.SetDefault(nl)
}

func Start() error {
	SetupLog()

	model, err := device.GetModel()
	if err != nil {
		slog.Error("cannot get model", "error", err.Error())
	}

	modelID, err := device.GetModelID()
	if err != nil {
		slog.Error("cannot get model id", "error", err.Error())
	}

	w1 := device.IsWalkmanOne()

	slog.Info("starting", "model", model.Device.Identification.Model, "fw", model.Device.Identification.Firmwareversion, "modelID", modelID, "walkmanOne", w1, "commit", Commit)

	f, err := os.OpenFile(SystemLogFile, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}

	b := bufio.NewReader(f)
	logEntries := make(chan string)
	errCh := make(chan error)

	go audioplayer.ErrHandler(errCh)

	go logreader.Read(b, logEntries, logreader.BufRead, errCh)

	r, err := resolver.New()
	if err != nil {
		return fmt.Errorf("cannot create resolver: %w", err)
	}

	clientString := fmt.Sprintf("%s@%s", name, Commit)
	deviceString := fmt.Sprintf("%s, fw %s", model.Device.Identification.Model, model.Device.Identification.Firmwareversion)
	if w1 {
		deviceString += ", walkmanOne"
	}

	scrobbler := &audioscrobbler.FileLog{}
	if err = scrobbler.New(clientString, deviceString); err != nil {
		return err
	}

	emitter := make(chan playerevents.PlayerEvent)
	go scrobbler.Listen(emitter, errCh)

	player := audioplayer.New().WithResolver(r).WithListenPercent(ListenPercent).WithPlayerEventEmitter(emitter)

	pp := parser.LogParser{}
	pp.Subscribe(player.Consumer())

	go func() {
		for {
			select {
			case line := <-logEntries:
				errCh <- pp.Parse(line)
			}
		}
	}()

	s := server.New("/tmp/scrobbler.sock")
	s.WithAudioPlayer(player)
	go s.Start()

	stop := make(chan struct{})
	player.Consume(stop, errCh)

	return nil
}
