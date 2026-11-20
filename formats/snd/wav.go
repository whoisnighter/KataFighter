package snd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// WAV is the format of a sound, read from its RIFF header.
type WAV struct {
	Format        uint16 // 1 = PCM, 3 = IEEE float, 0xFFFE = extensible
	Channels      uint16
	SampleRate    uint32
	BitsPerSample uint16
	DataSize      uint32 // bytes of sample data (clamped to what the file holds)
}

// Duration is the length of the sound.
func (w *WAV) Duration() time.Duration {
	frame := uint64(w.Channels) * uint64(w.BitsPerSample) / 8
	if frame == 0 || w.SampleRate == 0 {
		return 0
	}
	return time.Duration(uint64(w.DataSize) / frame * uint64(time.Second) / uint64(w.SampleRate))
}

func (w *WAV) String() string {
	ch := "mono"
	if w.Channels == 2 {
		ch = "stereo"
	} else if w.Channels != 1 {
		ch = fmt.Sprintf("%d channels", w.Channels)
	}
	return fmt.Sprintf("%d Hz %d-bit %s, %.2fs", w.SampleRate, w.BitsPerSample, ch, w.Duration().Seconds())
}

// ParseWAV reads the RIFF/WAVE header: the "fmt " chunk and the size of
// the "data" chunk. It does not decode samples.
func ParseWAV(b []byte) (*WAV, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("not a RIFF/WAVE file")
	}
	var w *WAV
	for p := 12; p+8 <= len(b); {
		id, n := string(b[p:p+4]), int64(binary.LittleEndian.Uint32(b[p+4:p+8]))
		body := b[p+8:]
		if n < int64(len(body)) {
			body = body[:n]
		}
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return nil, errors.New("WAV fmt chunk too short")
			}
			w = &WAV{
				Format:        binary.LittleEndian.Uint16(body[0:]),
				Channels:      binary.LittleEndian.Uint16(body[2:]),
				SampleRate:    binary.LittleEndian.Uint32(body[4:]),
				BitsPerSample: binary.LittleEndian.Uint16(body[14:]),
			}
		case "data":
			if w == nil {
				return nil, errors.New("WAV data chunk before fmt chunk")
			}
			w.DataSize = uint32(len(body))
			return w, nil
		}
		if n >= int64(len(b)) {
			break
		}
		p += 8 + int(n) + int(n&1) // chunks are word-aligned
	}
	if w == nil {
		return nil, errors.New("WAV has no fmt chunk")
	}
	return nil, errors.New("WAV has no data chunk")
}
