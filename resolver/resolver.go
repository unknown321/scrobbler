package resolver

import (
	"database/sql"
	"fmt"
	"log/slog"
	_ "modernc.org/sqlite"
	"path"
	"strings"
)

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

type Content struct {
	Artist         string
	Album          string
	Track          string
	TrackNumber    string
	Duration       uint
	Rating         bool // true if listened more than 50% of duration
	StartedAt      int64
	MusicBrainzTID string
	SampleRate     int
	Bitrate        int
	Channels       int
	BitDepth       int
	//filename       string
	Attempted bool
}

type DBContent struct {
	Artist      sql.NullString
	Album       sql.NullString
	Track       sql.NullString
	TrackNumber sql.NullString
	Duration    sql.NullInt32
}

var Skipped = "S"
var Listened = "L"

// Valid checks if fields required for scrobbler are filled
func (c *Content) Valid() bool {
	if c.Artist == "" || c.Track == "" || c.Duration == 0 || c.StartedAt == 0 {
		return false
	}

	return true
}

// produces scrobbler-compatible string
func (c *Content) String() string {
	rating := Skipped
	if c.Rating {
		rating = Listened
	}

	return fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\t%d\t",
		strings.ReplaceAll(c.Artist, "\t", ""),
		strings.ReplaceAll(c.Album, "\t", ""),
		strings.ReplaceAll(c.Track, "\t", ""),
		strings.ReplaceAll(c.TrackNumber, "\t", ""),
		c.Duration,
		rating,
		c.StartedAt,
	)
}

type DBResolver struct {
	db *sql.DB
}

type Resolver interface {
	Resolve(string) (*Content, error)
}

func New() (*DBResolver, error) {
	r := &DBResolver{}
	var err error

	r.db, err = sql.Open("sqlite", "/db/MTPDB.dat")
	if err != nil {
		return nil, fmt.Errorf("cannot open db: %w", err)
	}

	err = r.db.Ping()
	if err != nil {
		return nil, fmt.Errorf("cannot ping db: %w", err)
	}

	return r, nil
}

func (r *DBResolver) GetFileParams(objectId int32, akey int) (int, error) {
	rows, err := r.db.Query("SELECT value from object_ext_int where object_id = :object_id and akey= :akey",
		sql.Named("object_id", objectId), sql.Named("akey", akey),
	)

	if err != nil {
		return 0, fmt.Errorf("cannot query db: %w", err)
	}
	defer rows.Close()

	var result sql.NullInt32
	for rows.Next() {
		if err = rows.Scan(&result); err != nil {
			return 0, fmt.Errorf("cannot scan: %w", err)
		}
	}

	return int(result.Int32), nil
}

// Resolve takes uri string and returns content metadata for that uri
// content is matched on filename and its parent directory
func (r *DBResolver) Resolve(uri string) (*Content, error) {
	dir, filename := path.Split(uri)
	directory := path.Base(dir)

	rows, err := r.db.Query("SELECT ob.object_id, a.value, alb.value, ob.title, ob.series_no, info.value from object_body ob "+
		"join artists a on a.id = ob.artist_id "+
		"join albums alb on alb.id = ob.album_id "+
		"join object_ext_int info on info.object_id = ob.object_id "+
		"join object_body ob2 on ob2.object_id = ob.parent_id "+
		"where ob.filename = :filename and ob2.title = :directory and info.akey=12;",
		sql.Named("filename", filename), sql.Named("directory", directory),
	)

	if err != nil {
		return nil, fmt.Errorf("cannot query db: %w", err)
	}
	defer rows.Close()

	dbc := &DBContent{}
	var objectId sql.NullInt32

	for rows.Next() {
		if err = rows.Scan(&objectId, &dbc.Artist, &dbc.Album, &dbc.Track, &dbc.TrackNumber, &dbc.Duration); err != nil {
			return &Content{}, fmt.Errorf("cannot scan row: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	c := &Content{}
	if dbc.Artist.Valid {
		c.Artist = dbc.Artist.String
	}

	if dbc.Duration.Valid {
		c.Duration = uint(dbc.Duration.Int32) / 1000 // to seconds
	}

	if dbc.Track.Valid {
		c.Track = dbc.Track.String
	}

	if dbc.Album.Valid {
		c.Album = dbc.Album.String
	}

	if dbc.TrackNumber.Valid {
		c.TrackNumber = dbc.TrackNumber.String
	}

	if objectId.Valid {
		c.SampleRate, err = r.GetFileParams(objectId.Int32, 16)
		if err != nil {
			slog.Error("failed to get sample rate", "error", err.Error(), "object_id", objectId.Int32)
		}

		c.Bitrate, err = r.GetFileParams(objectId.Int32, 19)
		if err != nil {
			slog.Error("failed to get bitrate", "error", err.Error(), "object_id", objectId.Int32)
		}

		c.Channels, err = r.GetFileParams(objectId.Int32, 17)
		if err != nil {
			slog.Error("failed to get channels", "error", err.Error(), "object_id", objectId.Int32)
		}

		c.BitDepth, err = r.GetFileParams(objectId.Int32, 78)
		if err != nil {
			slog.Error("failed to get bit depth", "error", err.Error(), "object_id", objectId.Int32)
		}
	}

	c.Rating = false

	return c, nil
}
