package parser

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestGetContentPath(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				s: "I/hagodaemon(  281): 20240413 180234.150834 [INFO] [DmcOmxDemuxerCmp.c:2487] [tid:2350] " + ContentURIMarker + "/data/mnt/internal/MUSIC/Albums/1.flac",
			},
			want: "/data/mnt/internal/MUSIC/Albums/1.flac",
		},
		{
			name: "log is slightly off",
			args: args{
				s: "I/hagodaemon(  281): 20240413 180234.150834 [INFO] [DmcOmxDemuxerCmp.c:2487] [tid:2350] content URI invalid:",
			},
			want: "",
		},
		{
			name: "log is totally off",
			args: args{
				s: "I/hagodaemon(  281): 20240413 180648.385190 [INFO] [DmcOmxDemuxerCmp.c:5568] [tid:2350] componentOnStateChange: [OMX_StateIdle]->[OMX_StateLoaded]",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetContentPath(tt.args.s); got != tt.want {
				t.Errorf("GetContentPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndOfStream(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				s: "20240414 035742.052219 [INFO] [DmcAndroidAudioRendererCmp.c:1305] [tid:4859] EOS received. nFilledLen = [0], nTimeStamp = [224258321]",
			},
			want: "1",
		},
		{
			name: "invalid",
			args: args{
				s: "whatever",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EndOfStream(tt.args.s); got != tt.want {
				t.Errorf("EndOfStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPreparedTrack(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				s: "20240414 035736.608085 [INFO] [GapPlayerCmdHandlerPlay.c:533] [tid:527] Preparing next track.",
			},
			want: "1",
		},
		{
			name: "invalid",
			args: args{
				s: "help",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PreparedTrack(tt.args.s); got != tt.want {
				t.Errorf("PreparedTrack() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPlayerState(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				s: "I/hagodaemon(  281): 20240415 080125.728321 [INFO] [DmcAndroidAudioRendererCmp.c:1567] [tid:1675] componentOnStateChange: [OMX_StateLoaded]->[OMX_StateIdle]",
			},
			want: "[OMX_StateLoaded]->[OMX_StateIdle]",
		},
		{
			name: "invalid, wrong string",
			args: args{
				s: "I/hagodaemon(  281): 20240415 080725.774798 [INFO] [DmcOmxDemuxerCmp.c:5568] [tid:1672] asdasd",
			},
			want: "",
		},
		//{
		//	name: "raw from log, invalid",
		//	args: args{
		//		s: "\x04hagodaemon\t20240415 124530.562977 [INFO] [DmcOmxDemuxerCmp.c:5568] [tid:1942] componentOnStateChange: [OMX_StateLoaded]->[OMX_StateIdle]",
		//	},
		//	want: "",
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPlayerState(tt.args.s); got != tt.want {
				t.Errorf("GetPlayerState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func receive(cancel context.CancelFunc, c chan Event, res *[]Event) {
	m := sync.Mutex{}
Loop:
	for {
		select {
		case e := <-c:
			m.Lock()
			*res = append(*res, e)
			m.Unlock()
			if cap(c) == len(*res) {
				cancel()
				break Loop
			}
		}
	}
}

func TestLogParser_Parse(t *testing.T) {
	type args struct {
		expectedEvents int
		filename       string
		lines          []string
	}
	type want []Event

	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "prepared track event",
			args: args{
				expectedEvents: 1,
				lines:          []string{PreparedTrackMarker},
			},
			wantErr: false,
			want:    []Event{EventPreparing{}},
		},
		{
			name: "content uri event",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{ContentURIMarker + "/content"},
			},
			want:    []Event{EventContentURI{"/content"}},
			wantErr: false,
		},

		{
			name: "end of stream event",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{EndOfStreamMarker},
			},
			want:    []Event{EventEndOfStream{}},
			wantErr: false,
		},
		{
			name: "player state change event",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{PlayerStateMarker + "[OMX_StateExecuting]->[OMX_StatePause]"},
			},
			want: []Event{EventPlayerStateChange{
				Before: "OMX_StateExecuting",
				After:  "OMX_StatePause",
			}},
			wantErr: false,
		},
		{
			name: "storage mounted event",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{StorageMountedMarker},
			},
			want:    []Event{EventStorageMounted{}},
			wantErr: false,
		},
		{
			name: "storage unmounting event",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{StorageUnmountingMarker},
			},
			want:    []Event{EventStorageUnmounting{}},
			wantErr: false,
		},
		{
			name: "track created",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{"asdasd" + SoundServiceTrackSubstring + "trackID" + TrackCreatedMarker},
			},
			want:    []Event{EventTrackCreated{TrackID: "trackID"}},
			wantErr: false,
		},
		{
			name: "track destroyed",
			args: args{
				expectedEvents: 1,
				filename:       "",
				lines:          []string{SoundServiceTrackSubstring + "trackID" + TrackDestroyedMarker},
			},
			want:    []Event{EventTrackDestroyed{TrackID: "trackID"}},
			wantErr: false,
		},
		{
			name: "many events",
			args: args{
				expectedEvents: 107,
				filename:       "test/many_events.log",
				lines:          nil,
			},
			want: []Event{
				EventStorageMounted{},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_3_1"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_3_1"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/Paddy Fahey's.dsf"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_3_2"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_3_2"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/12 Sadeness (Meditation).ape"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_3"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventEndOfStream{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_3"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/Snowflake.flac"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_4"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac"},
				EventEndOfStream{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_4"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/07 The Voice & The Snake.flac"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_5"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventEndOfStream{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_5"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/01. Thunderstruck.mp3"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_6"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPreparing{},
				EventEndOfStream{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateExecuting"},
				EventEndOfStream{},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateExecuting", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StatePause", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StateLoaded"},
				EventTrackDestroyed{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_5_6"},
				EventContentURI{URI: "/data/mnt/internal/MUSIC/Don't Drift Too Far.dsf"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateLoaded", After: "OMX_StateIdle"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventPlayerStateChange{Before: "OMX_StateIdle", After: "OMX_StatePause"},
				EventTrackCreated{TrackID: "TK_MUSIC_PID_312_PKT_131072_QUE_3_7"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mychan := make(chan Event, tt.args.expectedEvents)
			subs := []chan Event{mychan}

			l := &LogParser{subs: subs}

			res := []Event{}
			ctx, cancel := context.WithCancel(context.Background())
			go receive(cancel, subs[0], &res)

			lines := tt.args.lines

			if tt.args.filename != "" {
				data, err := os.ReadFile(tt.args.filename)
				if err != nil {
					t.Errorf("cannot read test data: %s", err.Error())
				}

				lines = strings.Split(string(data), "\n")
			}

			for _, line := range lines {
				if err := l.Parse(line); (err != nil) != tt.wantErr {
					t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

		Wait:
			for {
				select {
				case <-ctx.Done():
					break Wait
				}
			}

			if !slices.Equal(res, tt.want) {
				t.Errorf("unexpected events: %s\n", cmp.Diff(res, tt.want))
			}
		})
	}
}
