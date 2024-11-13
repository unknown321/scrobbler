package playerevents

import "scrobbler/resolver"

type PlayerEvent interface {
	String()
}

type PlayerEventTrackListened struct {
	Content resolver.Content
}

func (pe PlayerEventTrackListened) String() {}
