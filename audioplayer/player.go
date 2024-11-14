package audioplayer

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
	"reflect"
	"scrobbler/audioscrobbler"
	"scrobbler/parser"
	"scrobbler/playerevents"
	"scrobbler/resolver"
	"strings"
	"sync"
	"time"
)

const (
	// StateStart is an internal scrobbler-only state
	StateStart = iota
	// StateStorageUnmounted is an internal scrobbler-only state
	// no disc action allowed!
	StateStorageUnmounted
	// StateStorageMounted is an internal scrobbler-only state
	// disc action allowed after this state
	StateStorageMounted
	StateWaitForResources
	StatePause
	StateExecuting
	StateUnknown
	StateInvalid
	StateLoaded
	StateIdle
)

var State = map[string]int{
	"OMX_StateWaitForResources": StateWaitForResources,
	"OMX_StatePause":            StatePause,
	"OMX_StateExecuting":        StateExecuting,
	"OMX_StateUnknown":          StateUnknown,
	"OMX_StateInvalid":          StateInvalid,
	"OMX_StateLoaded":           StateLoaded,
	"OMX_StateIdle":             StateIdle,
}

var StateByID = map[int]string{
	StateStart:            "ScrobblerStart",
	StateStorageUnmounted: "ScrobblerStorageUnmounted",
	StateStorageMounted:   "ScrobblerStorageMounted",
	StateWaitForResources: "OMX_StateWaitForResources",
	StatePause:            "OMX_StatePause",
	StateExecuting:        "OMX_StateExecuting",
	StateUnknown:          "OMX_StateUnknown",
	StateInvalid:          "OMX_StateInvalid",
	StateLoaded:           "OMX_StateLoaded",
	StateIdle:             "OMX_StateIdle",
}

var BeepIgnore = "WM_BEEP"

type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (*realClock) Now() time.Time                         { return time.Now() }
func (*realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type Track struct {
	ContentURI string
	PlayingFor int
	TrackID    string
}

// AudioPlayer tracks audio player state by consuming log entries
type AudioPlayer struct {
	State                 int
	StateBefore           int
	CurrentTrack          *Track // currently playing file uri + playing duration
	NextTrack             *Track
	Preparing             bool              // if true, next event with content uri belongs to next track
	CurrentContent        *resolver.Content // database info for current track
	consumer              chan parser.Event
	emitter               chan playerevents.PlayerEvent
	minimumListenDuration int
	divider               int // listened = duration/divider
	resolver              resolver.Resolver
	scrobbler             audioscrobbler.Log
	lock                  sync.Mutex
	tickDuration          time.Duration
	clock                 Clock
}

func New() *AudioPlayer {
	p := &AudioPlayer{
		State:          StateStart,
		StateBefore:    StateStart,
		CurrentTrack:   &Track{},
		NextTrack:      &Track{},
		Preparing:      false,
		CurrentContent: &resolver.Content{},
		consumer:       make(chan parser.Event),
		divider:        2,
		lock:           sync.Mutex{},
		tickDuration:   time.Second,
		emitter:        make(chan playerevents.PlayerEvent),
		clock:          &realClock{},
	}

	return p
}

func (p *AudioPlayer) WithResolver(r resolver.Resolver) *AudioPlayer {
	p.resolver = r
	return p
}

func (p *AudioPlayer) Resolve(uri string) (*resolver.Content, error) {
	if uri == "" {
		slog.Debug("empty uri")
		return &resolver.Content{}, nil
	}

	if strings.Contains(uri, BeepIgnore) {
		slog.Debug("ignoring beep")
		return &resolver.Content{}, nil
	}

	if p.resolver == nil {
		return &resolver.Content{}, fmt.Errorf("no resolver provided")
	}

	var err error
	var c *resolver.Content
	if c, err = p.resolver.Resolve(uri); err != nil {
		return nil, err
	}

	slog.Debug("resolved", "track", c.Track, "artist", c.Artist, "uri", uri)

	return c, nil
}

func (p *AudioPlayer) WithClock(clock Clock) *AudioPlayer {
	p.clock = clock
	return p
}

func (p *AudioPlayer) WithTickDuration(duration time.Duration) *AudioPlayer {
	p.tickDuration = duration
	return p
}

func (p *AudioPlayer) WithScrobbler(l audioscrobbler.Log) *AudioPlayer {
	p.scrobbler = l
	return p
}

func (p *AudioPlayer) WithListenPercent(percent int) *AudioPlayer {
	if percent <= 0 || percent > 100 {
		percent = 50
	}

	p.divider = 100 / percent

	return p
}

func (p *AudioPlayer) WithState(state int) *AudioPlayer {
	p.State = state
	return p
}

func (p *AudioPlayer) WithContent(c *resolver.Content) *AudioPlayer {
	p.CurrentContent = c
	return p
}

func (p *AudioPlayer) WithNextTrack(uri string, playingFor int) *AudioPlayer {
	p.NextTrack.ContentURI = uri
	p.NextTrack.PlayingFor = playingFor
	return p
}

func (p *AudioPlayer) WithCurrentTrack(uri string, playingFor int) *AudioPlayer {
	p.CurrentTrack.ContentURI = uri
	p.CurrentTrack.PlayingFor = playingFor
	return p
}

func (p *AudioPlayer) WithPlayerEventEmitter(peCh chan playerevents.PlayerEvent) *AudioPlayer {
	p.emitter = peCh
	return p
}

func (p *AudioPlayer) PlayerEventEmitter() chan playerevents.PlayerEvent {
	return p.emitter
}

func (p *AudioPlayer) Consumer() *chan parser.Event {
	return &p.consumer
}

func ErrHandler(e chan error) {
	var err error
	var ok bool
Loop:
	for {
		select {
		case err, ok = <-e:
			if !ok {
				break Loop
			}

			if err != nil {
				slog.Error("consume error", "error", err.Error())
			}
		}
	}
}

func (p *AudioPlayer) SetContentURI(uri string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// current track has NOT been destroyed
	if p.Preparing && p.CurrentTrack.ContentURI != "" {
		p.NextTrack.ContentURI = uri
		p.Preparing = false
		return
	}

	p.CurrentTrack.ContentURI = uri
	p.CurrentTrack.PlayingFor = 0
}

// Stop doesn't send listened events, only Destroy does
//
// bad case:
// - loop track
// - seek more than half of the track, so it won't be "listened"
// track doesn't get destroyed, no "skipped" event fired
func (p *AudioPlayer) Stop() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.CurrentTrack.PlayingFor = 0
	p.CurrentContent.Rating = false
	p.CurrentContent.StartedAt = 0
	p.CurrentContent.Attempted = false

	if p.NextTrack.ContentURI != "" {
		p.CurrentTrack.ContentURI = p.NextTrack.ContentURI
	}
}

func (p *AudioPlayer) Close() {
	p.lock.Lock()
	defer p.lock.Unlock()
}

// DestroyTrack resets all info current tracks
//
// At this point current track has been destroyed, there is no info about prepared track yet
func (p *AudioPlayer) DestroyTrack(s string) {
	slog.Debug("destroyed track %s", "track", s)
	if !p.CurrentContent.Rating && p.CurrentContent.Valid() && p.CurrentTrack.PlayingFor > 2 {
		e := playerevents.PlayerEventTrackListened{Content: *p.CurrentContent}
		p.emitter <- e
		slog.Info("sent to scrobbler as skipped", "uri", p.CurrentTrack.ContentURI)
	}

	p.CurrentContent = &resolver.Content{
		Artist:         "",
		Album:          "",
		Track:          "",
		TrackNumber:    "",
		Duration:       0,
		Rating:         false,
		StartedAt:      0,
		MusicBrainzTID: "",
		Attempted:      false,
	}

	if p.CurrentTrack.TrackID == s {
		p.CurrentTrack.TrackID = ""
		p.CurrentTrack.ContentURI = ""
		p.CurrentTrack.PlayingFor = 0
	}

	if p.NextTrack.TrackID == s {
		p.NextTrack.TrackID = ""
		p.NextTrack.ContentURI = ""
		p.NextTrack.PlayingFor = 0
	}
}

// CreateTrack event happens right before track starts playing
// so let's assume it is related to current track
func (p *AudioPlayer) CreateTrack(s string) {
	p.CurrentTrack.TrackID = s
}

var ErrUnknownEvent = errors.New("unknown event")

func (p *AudioPlayer) Consume(stop chan struct{}, errCh chan error) {
	ticker := time.NewTicker(p.tickDuration)
	defer ticker.Stop()

Consume:
	for {
		select {

		case event := <-p.consumer:
			slog.Debug("player event in", "event", reflect.TypeOf(event).String(), "data", fmt.Sprintf("%+v", event))

			switch event.(type) {
			case parser.EventPlayerStateChange:
				ee := event.(parser.EventPlayerStateChange)
				before := State[ee.Before]
				after := State[ee.After]
				errCh <- p.SetState(before, after)
			case parser.EventStorageUnmounting:
				errCh <- p.SetState(p.StateBefore, StateStorageUnmounted)
				p.Stop()
			case parser.EventStorageMounted:
				errCh <- p.SetState(p.StateBefore, StateStorageMounted)
			case parser.EventContentURI:
				ee := event.(parser.EventContentURI)
				p.SetContentURI(ee.URI)
			case parser.EventEndOfStream:
				p.Stop()
			case parser.EventPreparing:
				p.Preparing = true
			case parser.EventTrackDestroyed:
				ee := event.(parser.EventTrackDestroyed)
				p.DestroyTrack(ee.TrackID)
			case parser.EventTrackCreated:
				ee := event.(parser.EventTrackCreated)
				p.CreateTrack(ee.TrackID)
			default:
				errCh <- errors.Join(ErrUnknownEvent, errors.New(reflect.TypeOf(event).String()))
			}
		case <-ticker.C:
			if err2 := p.Tick(); err2 != nil {
				errCh <- err2
			}
		case <-stop:
			break Consume
		}
	}

	p.Close()
	ticker.Stop()
}

var ErrStorageUnmounted = errors.New("attempting to set state while storage is unmounted, ignoring")

func (p *AudioPlayer) SetState(before int, after int) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.State == StateStorageUnmounted {
		if after != StateStorageMounted {
			return ErrStorageUnmounted
		}
	}

	if after == StateStorageUnmounted || after == StateStorageMounted {
		p.StateBefore = p.State
		p.State = after
		return nil
	}

	if p.State == StateStart {
		p.StateBefore = after
		p.State = after
		return nil
	}

	if p.State != before && p.State != StateStorageMounted && p.State != StateStorageUnmounted {
		return nil
	}

	if after == StateLoaded {
		p.Preparing = false
	}

	p.StateBefore = p.State
	p.State = after

	return nil
}

// Tick ticks every second
//
// If you change tracks faster than one per second, only last one will be recorded
func (p *AudioPlayer) Tick() error {
	var err error

	if !p.CurrentContent.Attempted && p.CurrentTrack.ContentURI != "" {
		slog.Debug("resolving", "contentURI", p.CurrentTrack.ContentURI)
		if p.CurrentContent, err = p.Resolve(p.CurrentTrack.ContentURI); err != nil {
			p.CurrentContent.Attempted = true
			return err
		}

		p.CurrentContent.Attempted = true
		p.minimumListenDuration = int(p.CurrentContent.Duration) / p.divider
		p.CurrentContent.Rating = false
	}

	track := ""
	if p.CurrentContent.Valid() {
		track = p.CurrentContent.Track
	}

	slog.Debug("status", "track", path.Base(p.CurrentTrack.ContentURI), "contentTitle", track, "elapsed", p.CurrentTrack.PlayingFor, "state", StateByID[p.State], "min", p.minimumListenDuration)

	if p.State != StateExecuting {
		return nil
	}

	if p.CurrentContent.StartedAt == 0 {
		p.CurrentContent.StartedAt = p.clock.Now().Unix()
	}

	p.CurrentTrack.PlayingFor++

	if !p.CurrentContent.Valid() {
		slog.Debug("invalid track", "content", fmt.Sprintf("%+v", p.CurrentContent), "uri", p.CurrentTrack.ContentURI)
		return nil
	}

	if (p.CurrentTrack.PlayingFor >= p.minimumListenDuration) && !p.CurrentContent.Rating {
		p.CurrentContent.Rating = true
		event := playerevents.PlayerEventTrackListened{Content: *p.CurrentContent}
		p.emitter <- event

		slog.Info("sent to scrobbler", "track", p.CurrentContent.Track, "listened", p.CurrentContent.Rating, "for", p.CurrentTrack.PlayingFor)
	}

	return nil
}
