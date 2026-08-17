package video

import (
	"bytes"
	"testing"
)

func TestDetectContainerRecognisesKnownSignatures(t *testing.T) {
	cases := map[string]struct {
		header      []byte
		contentType string
	}{
		"mp4": {
			header:      []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00"),
			contentType: "video/mp4",
		},
		"quicktime is also iso base media": {
			header:      []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x02\x00"),
			contentType: "video/mp4",
		},
		"webm": {
			header:      append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{0}, 16)...),
			contentType: "video/webm",
		},
		"avi": {
			header:      []byte("RIFF\x00\x00\x00\x00AVI LIST"),
			contentType: "video/x-msvideo",
		},
		"mpeg program stream": {
			header:      append([]byte{0x00, 0x00, 0x01, 0xBA}, bytes.Repeat([]byte{0}, 16)...),
			contentType: "video/mpeg",
		},
		"flv": {
			header:      append([]byte("FLV\x01"), bytes.Repeat([]byte{0}, 16)...),
			contentType: "video/x-flv",
		},
		"ogg": {
			header:      append([]byte("OggS"), bytes.Repeat([]byte{0}, 16)...),
			contentType: "video/ogg",
		},
		"mpeg transport stream": {
			header:      transportStreamHeader(),
			contentType: "video/mp2t",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			contentType, recognised := DetectContainer(testCase.header)
			if !recognised {
				t.Fatalf("expected %s to be recognised", name)
			}
			if contentType != testCase.contentType {
				t.Fatalf("expected %q, got %q", testCase.contentType, contentType)
			}
		})
	}
}

func TestDetectContainerRejectsNonVideoPayloads(t *testing.T) {
	cases := map[string][]byte{
		"shell script":       []byte("#!/bin/sh\necho hello\n"),
		"elf binary":         []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
		"png":                []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"),
		"empty":              {},
		"truncated ftyp box": []byte("\x00\x00\x00\x20ftyp"),
		// A lone 0x47 is a common byte; without a second sync byte one packet
		// later it must not be taken for a transport stream.
		"single transport stream sync byte": append([]byte{0x47}, bytes.Repeat([]byte{0x00}, 200)...),
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			if contentType, recognised := DetectContainer(header); recognised {
				t.Fatalf("expected %s to be rejected, got %q", name, contentType)
			}
		})
	}
}

func transportStreamHeader() []byte {
	header := make([]byte, 189)
	header[0] = 0x47
	header[188] = 0x47
	return header
}
