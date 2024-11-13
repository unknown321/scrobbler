package logreader

import (
	"bufio"
	"bytes"
	"encoding/binary"
)

// LoggerEntryHeader taken from https://android.googlesource.com/platform/system/core/+/bcd37e67dbf1e420c41b7cbaa22142c14ec5d8fc/include/log/logger.h#30
type LoggerEntryHeader struct {
	Length uint16 /* length of the payload */
	Pad    uint16 /* no matter what, we get 2 bytes of padding */
	Pid    int32  /* generating process's pid */
	Tid    int32  /* generating process's tid */
	Sec    int32  /* seconds since Epoch */
	NSec   int32  /* nanoseconds */
}

var HeaderLength = 2 + 2 + 4 + 4 + 4 + 4

func BufRead(f *bufio.Reader, messages chan []byte) error {
	// https://android.googlesource.com/platform/system/core/+/bcd37e67dbf1e420c41b7cbaa22142c14ec5d8fc/include/log/logger.h#91
	// might be not the same version, but same idea
	// buffer on kernel side might be smaller, but it doesn't matter

	var err error
	var n int

	for {
		var buffer [5 * 1024]byte
		pp := buffer[:]
		/* driver guarantees we read exactly one full entry */

		n, err = f.Read(pp)

		if n > 0 && err == nil {
			messages <- pp
			continue
		} else {
			break
		}
	}

	return err
}

// Read reads from source using readFunc
//
// Get lines and errors from `entries` and `errCh` channels
func Read(source *bufio.Reader, entries chan string, readFunc func(*bufio.Reader, chan []byte) error, errCh chan error) {
	messages := make(chan []byte)

	go func() {
		for {
			b := <-messages
			if len(b) > 0 {
				l := &LoggerEntryHeader{}
				b = bytes.Trim(b, "\x00")
				buf := bytes.NewReader(b)

				err := binary.Read(buf, binary.LittleEndian, l)
				if err != nil {
					errCh <- err
					continue
				}

				// entry length specified in header doesn't match length after trimming null bytes
				// cut off the header, treat everything else as a message
				if l.Length > 0 {
					msg := b[HeaderLength:]
					// some messages have newlines in them, logcat somehow gets rid of them
					// so do it too
					msg = bytes.ReplaceAll(msg, []byte("\n"), []byte(" "))
					// process name is separated from message by null byte, replace by tab
					msg = bytes.ReplaceAll(msg, []byte("\x00"), []byte("\t"))
					entries <- string(msg)
				}
			}
		}
	}()

	if err := readFunc(source, messages); err != nil {
		errCh <- err
	}

	return
}
