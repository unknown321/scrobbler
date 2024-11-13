package audioplayer

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"os"
	"scrobbler/parser"
	"scrobbler/playerevents"
	"scrobbler/resolver"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type DumbResolver struct{}

func (d *DumbResolver) Resolve(s string) (*resolver.Content, error) {
	return &resolver.Content{
		Artist:         "artist",
		Album:          "album",
		Track:          s,
		TrackNumber:    "1",
		Duration:       10,
		Rating:         false,
		StartedAt:      0,
		MusicBrainzTID: "",
	}, nil
}

func ConsumeErrHandler(stop chan struct{}, errCh chan error, errors *[]error) {
	lock := sync.Mutex{}
	for {
		select {
		case e := <-errCh:
			lock.Lock()
			if e != nil {
				*errors = append(*errors, e)
			}
			lock.Unlock()
		case <-stop:
			return
		}
	}
}

func EmitterReceiver(peCh chan playerevents.PlayerEvent, events *[]playerevents.PlayerEvent, stop chan struct{}) {
	lock := sync.Mutex{}
	for {
		select {
		case e := <-peCh:
			lock.Lock()
			if e != nil {
				*events = append(*events, e)
			}
			lock.Unlock()
		case <-stop:
			return
		}
	}
}

type staticClock struct {
	internalTime int
}

func (s *staticClock) Now() time.Time {
	res := time.Unix(int64(12345+s.internalTime), 0)
	s.internalTime++
	return res
}

func (*staticClock) After(d time.Duration) <-chan time.Time {
	panic("implement me")
}

func TestAudioPlayer_Consume(t *testing.T) {
	type fields struct {
		AudioPlayer *AudioPlayer
		Events      []parser.Event
		Filename    string
	}

	type want struct {
		Player       *AudioPlayer
		Errors       []error
		PlayerEvents []playerevents.PlayerEvent
	}

	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "consume state event",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}),
				Events:      []parser.Event{parser.EventPlayerStateChange{Before: StateByID[StateStart], After: StateByID[StateExecuting]}},
			},
			want: want{Player: New().WithState(StateExecuting), Errors: []error{}},
		},
		{
			name: "storage unmounted",
			fields: fields{
				AudioPlayer: New().WithState(StateExecuting).WithResolver(&DumbResolver{}),
				Events:      []parser.Event{parser.EventStorageUnmounting{}},
			},
			want: want{
				Player: New().WithState(StateStorageUnmounted),
				Errors: []error{},
			},
		},
		{
			name: "storage unmounted, trying to set wrong state",
			fields: fields{
				AudioPlayer: New().WithState(StateStorageUnmounted).WithResolver(&DumbResolver{}),
				Events:      []parser.Event{parser.EventPlayerStateChange{Before: StateByID[StateStart], After: StateByID[StateExecuting]}},
			},
			want: want{
				Player: New().WithState(StateStorageUnmounted),
				Errors: []error{ErrStorageUnmounted},
			},
		},
		{
			name: "set content uri in preparing state",
			fields: fields{
				AudioPlayer: New().WithCurrentTrack("/ct", 0).WithResolver(&DumbResolver{}),
				Events:      []parser.Event{parser.EventPreparing{}, parser.EventContentURI{URI: "/test"}},
			},
			want: want{
				Player: New().WithNextTrack("/test", 0).WithCurrentTrack("/ct", 0),
				Errors: []error{},
			},
		},
		{
			name: "set content uri",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}),
				Events:      []parser.Event{parser.EventContentURI{URI: "/test"}},
			},
			want: want{
				Player: New().WithCurrentTrack("/test", 0),
				Errors: []error{},
			},
		},
		{
			name: "automatic change, event fired on first track",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/automatic_change.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/2.flac", 0).WithState(StatePause),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:         "artist",
						Album:          "album",
						Track:          "/data/mnt/internal/MUSIC/1.flac",
						TrackNumber:    "1",
						Duration:       10,
						Rating:         true,
						StartedAt:      12345,
						MusicBrainzTID: "",
						Attempted:      true,
					}},
				},
			},
		},
		{
			name: "loop track 3 times, no stop",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/loop.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/71. Pop Team Epicrimson.mp3", 0).WithState(StateExecuting),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/71. Pop Team Epicrimson.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12345,
						Attempted:   true,
					}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/71. Pop Team Epicrimson.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12346,
						Attempted:   true,
					}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/71. Pop Team Epicrimson.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12347,
						Attempted:   true,
					}},
				},
			},
		},
		{
			name: "manual track change",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithTickDuration(time.Millisecond * 100).WithClock(&staticClock{}),
				Filename:    "test/manual_track_change.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/2.flac", 0).WithState(StateExecuting),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{playerevents.PlayerEventTrackListened{Content: resolver.Content{
					Artist:         "artist",
					Album:          "album",
					Track:          "/data/mnt/internal/MUSIC/2.flac",
					TrackNumber:    "1",
					Duration:       10,
					Rating:         true,
					StartedAt:      12345,
					MusicBrainzTID: "",
					Attempted:      true,
				}}},
			},
		},
		{
			name: "manual track change after preloading",
			fields: fields{
				AudioPlayer: New().WithCurrentTrack("/whatever", 10).WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/manual_track_change_after_preloading.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/03.mp3", 0).WithState(StateExecuting),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{playerevents.PlayerEventTrackListened{Content: resolver.Content{
					Artist:      "artist",
					Album:       "album",
					Track:       "/data/mnt/internal/MUSIC/01.mp3",
					TrackNumber: "1",
					Duration:    10,
					Rating:      true,
					StartedAt:   12345,
					Attempted:   true,
				}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/03.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12346,
						Attempted:   true,
					}}},
			},
		},
		{
			name: "previous button pressed once, same track playing, scrobbler event emitted",
			fields: fields{
				AudioPlayer: New().WithCurrentTrack("/whatever", 10).WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/prev_once.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/1.flac", 0).WithState(StateExecuting),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{playerevents.PlayerEventTrackListened{Content: resolver.Content{
					Artist:      "artist",
					Album:       "album",
					Track:       "/data/mnt/internal/MUSIC/1.flac",
					TrackNumber: "1",
					Duration:    10,
					Rating:      true,
					StartedAt:   12345,
					Attempted:   true,
				}}},
			},
		},
		{
			name: "manual switch to previous track by pressing previous button, previous track plays long enough for scrobbler event",
			fields: fields{
				AudioPlayer: New().WithCurrentTrack("/whatever", 10).WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/previous_track_manual.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/99.mp3", 0).WithState(StateExecuting),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{playerevents.PlayerEventTrackListened{Content: resolver.Content{
					Artist:      "artist",
					Album:       "album",
					Track:       "/data/mnt/internal/MUSIC/99.mp3",
					TrackNumber: "1",
					Duration:    10,
					Rating:      true,
					StartedAt:   12345,
					Attempted:   true,
				}}},
			},
		},
		{
			name: "regular play, 6 songs, no loop",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 10),
				Filename:    "test/regular_play_6_songs_no_loop.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf", 0).WithState(StatePause),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12345,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Paddy Fahey's.dsf",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12346,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/12 Sadeness (Meditation).ape",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12347,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Snowflake.flac",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12348,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12349,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/01. Thunderstruck.mp3",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12350,
							Attempted:   true,
						},
					},
				},
			},
		},
		{
			name: "regular play, 6 songs, no loop, shuffle",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 10),
				Filename:    "test/regular_play_6_songs_no_loop_shuffle.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf", 0).WithState(StatePause),
				Errors: []error{},
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/12 Sadeness (Meditation).ape",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12345,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12346,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Snowflake.flac",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12347,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/01. Thunderstruck.mp3",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12348,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12349,
							Attempted:   true,
						},
					},
					playerevents.PlayerEventTrackListened{
						Content: resolver.Content{
							Artist:      "artist",
							Album:       "album",
							Track:       "/data/mnt/internal/MUSIC/Paddy Fahey's.dsf",
							TrackNumber: "1",
							Duration:    10,
							Rating:      true,
							StartedAt:   12350,
							Attempted:   true,
						},
					},
				},
			},
		},
		{
			name: "regular play, track #2 skipped, 3 events sent",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 10),
				Filename:    "test/regular_play_track_skipped.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/01 - Resurrection.mp3", 0).WithState(StateExecuting),
				Errors: nil,
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12345,
						Attempted:   true,
					}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/03 - Bucovina [Haaksman & Haaksman Soca Bogle Mix] - Shantel.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      false,
						StartedAt:   12346,
						Attempted:   true,
					}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/01 - Resurrection.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12347,
						Attempted:   true,
					}},
				},
			},
		},
		{
			name: "song playing, usb connected, storage unmounted",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/playing_then_unmounted.log",
			},
			want: want{
				Player: New().WithState(StateStorageUnmounted),
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/01 - Resurrection.mp3",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12345,
						Attempted:   true,
					}},
				},
				Errors: []error{ErrStorageUnmounted, ErrStorageUnmounted, ErrStorageUnmounted, ErrStorageUnmounted, ErrStorageUnmounted, ErrStorageUnmounted},
			},
		},
		{
			name: "mounted storage, start playing",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 10).WithState(StateStorageMounted),
				Filename:    "test/mounted_then_playing.log",
			},
			want: want{
				Player: New().WithState(StateExecuting).WithCurrentTrack("/data/mnt/internal/MUSIC/01 - Resurrection.mp3", 10),
				Errors: nil,
				PlayerEvents: []playerevents.PlayerEvent{playerevents.PlayerEventTrackListened{Content: resolver.Content{
					Artist:      "artist",
					Album:       "album",
					Track:       "/data/mnt/internal/MUSIC/01 - Resurrection.mp3",
					TrackNumber: "1",
					Duration:    10,
					Rating:      true,
					StartedAt:   12345,
					Attempted:   true,
				}}},
			},
		},
		{
			name: "dsf loop",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/dsf_loop.log",
			},
			want: want{
				Player: New().WithState(StateExecuting).WithCurrentTrack("/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf", 0),
				Errors: nil,
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12345,
						Attempted:   true,
					}},
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12346,
						Attempted:   true,
					}},
				},
			},
		},
		{
			name: "bug: two ListenedEvents on track change",
			fields: fields{
				AudioPlayer: New().WithResolver(&DumbResolver{}).WithClock(&staticClock{}).WithTickDuration(time.Millisecond * 50),
				Filename:    "test/bug_double_event_on_track_switch.log",
			},
			want: want{
				Player: New().WithCurrentTrack("/data/mnt/internal/MUSIC/01 - Resurrection.mp3", 0).WithState(StatePause),
				Errors: nil,
				PlayerEvents: []playerevents.PlayerEvent{
					playerevents.PlayerEventTrackListened{Content: resolver.Content{
						Artist:      "artist",
						Album:       "album",
						Track:       "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac",
						TrackNumber: "1",
						Duration:    10,
						Rating:      true,
						StartedAt:   12345,
						Attempted:   true,
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := tt.fields.AudioPlayer.Consumer()
			stop1 := make(chan struct{})
			stop2 := make(chan struct{})
			stop3 := make(chan struct{})
			errCh := make(chan error)

			wg := &sync.WaitGroup{}

			wg.Add(1)
			go func() {
				tt.fields.AudioPlayer.Consume(stop1, errCh)
				wg.Done()
			}()

			errors := []error{}
			wg.Add(1)
			go func() {
				ConsumeErrHandler(stop2, errCh, &errors)
				wg.Done()
			}()

			playerEvents := []playerevents.PlayerEvent{}
			wg.Add(1)
			go func() {
				EmitterReceiver(tt.fields.AudioPlayer.PlayerEventEmitter(), &playerEvents, stop3)
				wg.Done()
			}()

			if tt.fields.Filename != "" {
				data, err := os.ReadFile(tt.fields.Filename)
				if err != nil {
					t.Errorf("cannot read test data: %s", err.Error())
				}
				lines := strings.Split(string(data), "\n")

				pp := parser.LogParser{}
				pp.Subscribe(tt.fields.AudioPlayer.Consumer())

				for _, line := range lines {
					err = pp.Parse(line)
				}
			} else {
				for _, e := range tt.fields.Events {
					*consumer <- e
				}
			}

			time.Sleep(time.Second)

			stop1 <- struct{}{}
			stop2 <- struct{}{}
			stop3 <- struct{}{}
			wg.Wait()

			if !slices.Equal(tt.want.Errors, errors) {
				t.Errorf("errors not equal: %s", cmp.Diff(tt.want.Errors, errors, cmpopts.EquateErrors()))
			}

			if tt.want.Player.State != tt.fields.AudioPlayer.State {
				t.Errorf("State mismatsh: %s, want %s", StateByID[tt.fields.AudioPlayer.State], StateByID[tt.want.Player.State])
			}

			if tt.want.Player.CurrentTrack.ContentURI != tt.fields.AudioPlayer.CurrentTrack.ContentURI {
				t.Errorf("current track contentURI mismatsh: %s, want %s", tt.fields.AudioPlayer.CurrentTrack.ContentURI, tt.want.Player.CurrentTrack.ContentURI)
			}

			if !slices.Equal(tt.want.PlayerEvents, playerEvents) {
				t.Errorf("player events not equal: %s",
					cmp.Diff(tt.want.PlayerEvents, playerEvents))
			}
		})
	}
}
