package audioscrobbler

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"reflect"
	"scrobbler/playerevents"
)

// https://web.archive.org/web/20170107015006/http://www.audioscrobbler.net/wiki/Portable_Player_Logging
// .scrobbler.log Explained
//
// Song played/skipped events are written to a text file in the root of the filesystem on the player, called: .scrobbler.log (note the first character of the file is a period)
//
// The first lines of .scrobbler.log should be header lines, indicated by the leading '#' character:
//
// #AUDIOSCROBBLER/1.1
// #TZ/[UNKNOWN|UTC]
// #CLIENT/<IDENTIFICATION STRING>
//
// Where 1.1 is the version for this file format (there may be further revisions to the format later) The TZ line tells audioscrobbler what timezone the player is in, so that the times songs were played can be adjusted correctly.
//
// If the device knows what timezone it is in, it must convert all logged times to UTC (aka GMT+0)
// eg: #TZ/UTC
// If the device knows the time, but no the timezone, then the software on the PC that syncs the device will adjust times based on the timezone setting of the PC. (as all submissions to audioscrobbler servers must be UTC)
// eg: #TZ/UNKNOWN
//
// <IDENTIFICATION STRING> should be replaced by the name/model of the hardware device and the revision of the software producing the log file.
//
// After the header lines, simply append one line of text for every song that is played or skipped.
//
// The following fields comprise each line, and are tab (\t) separated (strip any tab characters from the data):
//
// - artist name
// - album name (optional)
// - track name
// - track position on album (optional)
// - song duration in seconds
// - rating (L if listened at least 50% or S if skipped)
// - unix timestamp when song started playing
// - MusicBrainz Track ID (optional)
//
// lines should be terminated with \n
// Example
//
// (listened to enter sandman, skipped cowboys, listened to the pusher) :
//
// #AUDIOSCROBBLER/1.0
// #TZ/UTC
// #CLIENT/Rockbox h3xx 1.1
// Metallica        Metallica        Enter Sandman        1        365        L        1143374412        62c2e20a?-559e-422f-a44c-9afa7882f0c4?
// Portishead        Roseland NYC Live        Cowboys        2        312        S        1143374777        db45ed76-f5bf-430f-a19f-fbe3cd1c77d3
// Steppenwolf        Live        The Pusher        12        350        L        1143374779        58ddd581-0fcc-45ed-9352-25255bf80bfb?
//
// If the data for optional fields is not available to you, leave the field blank (\t\t).
// All strings should be written as UTF-8, although the file does not use a BOM.
// All fields except those marked (optional) above are required.
// If your device does not have a clock, you will not be able to supply a timestamp and cannot use this service.
// If any of the required fields are missing, then you do not have enough data to submit - do not write to the file.
// More infomation about the MusicBrainz Track ID is available at http://musicbrainz.org/docs/specs/metadata_tags.html
//

var Header = "#AUDIOSCROBBLER/1.1\n" +
	"#TZ/UNKNOWN\n"

var Filename = "/data/mnt/internal/.scrobbler.log"

type Log interface {
	New(client string, device string) error
	Add(s string) error
}

type FileLog struct {
	client string
	device string
}

func (l *FileLog) New(client string, device string) error {
	l.client = client
	l.device = device

	return nil
}

func (l *FileLog) Listen(c chan playerevents.PlayerEvent, errCh chan error) {
	for {
		select {
		case e := <-c:
			switch e.(type) {
			case playerevents.PlayerEventTrackListened:
				event := e.(playerevents.PlayerEventTrackListened)
				errCh <- l.Add(event.Content.String())
			default:
				errCh <- fmt.Errorf("unknown event: %s", reflect.TypeOf(e).String())
			}
		}
	}
}

func (l *FileLog) Add(s string) error {
	var err error
	var f *os.File

	_, err = os.Stat(Filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Debug("creating new log", "path", Filename)

			if f, err = os.Create(Filename); err != nil {
				return fmt.Errorf("cannot create new log file: %w", err)
			}

			h := Header + fmt.Sprintf("#CLIENT/%s on %s", l.client, l.device)
			if _, err = f.WriteString(h + "\n"); err != nil {
				return fmt.Errorf("cannot write log header: %w", err)
			}
		} else {
			return fmt.Errorf("cannot stat log file: %w", err)
		}
	}

	if f == nil {
		f, err = os.OpenFile(Filename, os.O_APPEND|os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("cannot open existing log file: %w", err)
		}
		defer f.Close()
	}

	if _, err = f.WriteString(s + "\n"); err != nil {
		return fmt.Errorf("cannot add log entry: %w", err)
	}

	return nil
}
