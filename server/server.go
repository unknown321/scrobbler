package server

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"scrobbler/audioplayer"
	"syscall"
)

type Server struct {
	socketAddr string
	player     *audioplayer.AudioPlayer
}

func New(addr string) *Server {
	return &Server{socketAddr: addr}
}

var CMDListBegin = []byte("noidle\ncommand_list_begin\n")
var CMDStatus = []byte("status\n")
var CMDCurrentSong = []byte("currentsong\n")
var CMDListEnd = []byte("command_list_end\nidle\n")

var CMDStatusBatch = append(CMDStatus, CMDCurrentSong...)
var CMDStatusBatchAll = bytes.Join([][]byte{CMDListBegin, CMDStatus, CMDCurrentSong, CMDListEnd}, []byte(""))

var ReplyOK = []byte("OK\n")

func (s *Server) WithAudioPlayer(player *audioplayer.AudioPlayer) *Server {
	s.player = player
	return s
}

var StateByID = map[int]string{
	audioplayer.StatePause:     "pause",
	audioplayer.StateExecuting: "play",
}

func (s *Server) Start() {
	socket, err := net.Listen("unix", s.socketAddr)
	if err != nil {
		slog.Error("cannot start server", "address", s.socketAddr, "err", err.Error())
		return
	}

	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(s.socketAddr)
		os.Exit(1)
	}()

	fmt.Printf("started socket server on %s\n", s.socketAddr)
	for {
		conn, err := socket.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// Handle the connection in a separate goroutine.
		go func(conn net.Conn) {
			defer conn.Close()
			// Create a buffer for incoming data.
			buf := make([]byte, 4096)

			//https: //pkg.go.dev/toolman.org/net/peercred ?
			//fmt.Printf("conn from: %s\n", conn.RemoteAddr().String())
			for {
				// Read data from the connection.
				n, err := conn.Read(buf)
				if err != nil {
					slog.Error("cannot read from socket", "err", err.Error())
					break
				}

				if n < 1 {
					continue
				}

				if bytes.Equal(CMDStatusBatchAll, buf[:n]) {
					state := ""
					var ok bool
					if state, ok = StateByID[s.player.State]; !ok {
						state = "stop"
					}

					res := fmt.Sprintf(
						"OK\n"+
							"volume: %d\n"+
							"state: %s\n"+
							"elapsed: %d\n"+
							"bitrate: %d\n"+
							"duration: %d\n"+
							"file: %s\n"+
							"audio: %d:%d:%d\n"+
							"Artist: %s\n"+
							"Album: %s\n"+
							"Title: %s\n"+
							"Track: %s\n"+
							//"Date: %s\n"+
							"OK\n",
						50,
						state,
						s.player.CurrentTrack.PlayingFor,
						s.player.CurrentContent.Bitrate/1000,
						s.player.CurrentContent.Duration,
						s.player.CurrentTrack.ContentURI,
						s.player.CurrentContent.SampleRate, s.player.CurrentContent.BitDepth, s.player.CurrentContent.Channels,
						s.player.CurrentContent.Artist,
						s.player.CurrentContent.Album,
						s.player.CurrentContent.Track,
						s.player.CurrentContent.TrackNumber,
					)
					conn.Write([]byte(res))
				}

				_, err = conn.Write(ReplyOK)
				if err != nil {
					slog.Error("cannot write to socket", "err", err.Error())
					break
				}
			}
		}(conn)
	}
}
