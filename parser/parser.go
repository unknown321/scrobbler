package parser

import (
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var ContentURIMarker = "] content URI: "

// PlayerStateSourceLineMarker is used to prevent double state change, because PlayerStateMarker is duplicated in logs
//var PlayerStateSourceLineMarker = "DmcAndroidAudioRendererCmp.c"

// PlayerStateMarker is duplicated by android audio renderer and current demuxer which change all the time
var PlayerStateMarker = "componentOnStateChange: "

var PreparedTrackMarker = "] Preparing next track."

var EndOfStreamMarker = "] EOS received. nFilledLen ="

var StorageUnmountingMarker = "storage[Internal], status[Unmounting]"
var StorageMountedMarker = "storage[Internal], status[Mounted]"
var TrackDestroyedMarker = "] has been destroyed"
var TrackCreatedMarker = "] has been created"
var SoundServiceTrackSubstring = "] Track["
var SleepForTestsMarker = "SLEEP FOR "

var Markers = map[string]func(string) string{
	ContentURIMarker:        GetContentPath,
	PlayerStateMarker:       GetPlayerState,
	PreparedTrackMarker:     PreparedTrack,
	EndOfStreamMarker:       EndOfStream,
	StorageUnmountingMarker: StorageUnmounting,
	StorageMountedMarker:    StorageMounted,
	TrackDestroyedMarker:    TrackDestroyed,
	TrackCreatedMarker:      TrackCreated,
	SleepForTestsMarker:     SleepForTests,
}

func SleepForTests(s string) string {
	if !strings.Contains(s, SleepForTestsMarker) {
		return ""
	}
	return s[len(SleepForTestsMarker):]

}

func EndOfStream(s string) string {
	if strings.Contains(s, EndOfStreamMarker) {
		return "1"
	}

	return ""
}

func PreparedTrack(s string) string {
	if strings.Contains(s, PreparedTrackMarker) {
		return "1"
	}

	return ""
}

func GetPlayerState(s string) string {
	if !strings.Contains(s, PlayerStateMarker) {
		return ""
	}

	end := strings.Index(s, PlayerStateMarker) + len(PlayerStateMarker)
	return s[end:]
}

func GetContentPath(s string) string {
	if !strings.Contains(s, ContentURIMarker) {
		return ""
	}

	start := strings.Index(s, ContentURIMarker)
	end := start + len(ContentURIMarker)

	return s[end:]
}

func StorageUnmounting(s string) string {
	if !strings.Contains(s, StorageUnmountingMarker) {
		return ""
	}

	return s
}

func StorageMounted(s string) string {
	if !strings.Contains(s, StorageMountedMarker) {
		return ""
	}

	return s
}

func TrackDestroyed(s string) string {
	if !strings.Contains(s, TrackDestroyedMarker) {
		return ""
	}

	start := strings.Index(s, SoundServiceTrackSubstring) + len(SoundServiceTrackSubstring)
	end := strings.Index(s[start+1:], "]") + start + 1
	trackID := s[start:end]

	return trackID
}

func TrackCreated(s string) string {
	if !strings.Contains(s, TrackCreatedMarker) {
		return ""
	}

	start := strings.Index(s, SoundServiceTrackSubstring) + len(SoundServiceTrackSubstring)
	end := strings.Index(s[start+1:], "]") + start + 1
	trackID := s[start:end]

	return trackID
}

type Event interface {
	String()
}

type EventPreparing struct{}

func (e EventPreparing) String() {}

type EventContentURI struct {
	URI string
}

func (e EventContentURI) String() {}

type EventEndOfStream struct{}

func (e EventEndOfStream) String() {}

type EventPlayerStateChange struct {
	Before string
	After  string
}

func (e EventPlayerStateChange) String() {}

type EventStorageMounted struct{}

func (e EventStorageMounted) String() {}

type EventStorageUnmounting struct{}

func (e EventStorageUnmounting) String() {}

type EventTrackDestroyed struct {
	TrackID string
}

func (EventTrackDestroyed) String() {}

type EventTrackCreated struct {
	TrackID string
}

func (EventTrackCreated) String() {}

type LogParser struct {
	subs []chan Event
}

func (l *LogParser) Subscribe(e *chan Event) {
	l.subs = append(l.subs, *e)
}

func (l *LogParser) Parse(s string) error {
	value := ""
	mark := ""
	for marker, v := range Markers {
		if strings.Contains(s, marker) {
			value = v(s)
			mark = marker
			break
		}
	}

	var event Event
	switch mark {
	case PreparedTrackMarker:
		event = EventPreparing{}
	case ContentURIMarker:
		event = EventContentURI{URI: value}
	case EndOfStreamMarker:
		event = EventEndOfStream{}
	case PlayerStateMarker:
		states := strings.Split(value, "->")
		if len(states) != 2 {
			return fmt.Errorf("cannot split player state in 2 by ->: %s; %s", value, s)
		}

		event = EventPlayerStateChange{
			Before: states[0][1 : len(states[0])-1],
			After:  states[1][1 : len(states[1])-1],
		}
	case StorageMountedMarker:
		event = EventStorageMounted{}
		slog.Debug("storage mounted")
	case StorageUnmountingMarker:
		event = EventStorageUnmounting{}
		slog.Debug("storage unmounting")
	case TrackDestroyedMarker:
		event = EventTrackDestroyed{TrackID: value}
	case TrackCreatedMarker:
		event = EventTrackCreated{TrackID: value}
	case SleepForTestsMarker:
		d, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			slog.Error("cannot parse")
		}

		time.Sleep(time.Millisecond * time.Duration(d))
	}

	if event == nil {
		return nil
	}

	for n, sub := range l.subs {
		slog.Debug("parser sending", "event", reflect.TypeOf(event).String(), "subscriber", n, "data", fmt.Sprintf("%+v", event))
		sub <- event
	}

	return nil
}
