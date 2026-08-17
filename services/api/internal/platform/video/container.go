package video

import "bytes"

// headerBytes is how much of the upload is inspected to identify the container.
// It has to cover the MPEG-TS check, which needs two sync bytes 188 apart.
const headerBytes = 512

// DetectContainer identifies a video container from the first bytes of an upload.
//
// The browser-supplied Content-Type is not evidence: it is trivially forged, and a
// storage endpoint that trusts it will happily accept an executable named
// "clip.mp4". What gets stored is what the bytes say they are, or nothing.
func DetectContainer(header []byte) (string, bool) {
	switch {
	case isISOBaseMedia(header):
		return "video/mp4", true
	case bytes.HasPrefix(header, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		// EBML: Matroska and WebM share a header; both are acceptable here.
		return "video/webm", true
	case isRIFFAVI(header):
		return "video/x-msvideo", true
	case bytes.HasPrefix(header, []byte{0x00, 0x00, 0x01, 0xBA}):
		return "video/mpeg", true
	case bytes.HasPrefix(header, []byte("FLV\x01")):
		return "video/x-flv", true
	case bytes.HasPrefix(header, []byte("OggS")):
		return "video/ogg", true
	case isMPEGTransportStream(header):
		return "video/mp2t", true
	default:
		return "", false
	}
}

// isISOBaseMedia covers MP4, M4V and QuickTime, which all carry an "ftyp" box at
// offset 4.
func isISOBaseMedia(header []byte) bool {
	return len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp"))
}

func isRIFFAVI(header []byte) bool {
	return len(header) >= 12 &&
		bytes.HasPrefix(header, []byte("RIFF")) &&
		bytes.Equal(header[8:12], []byte("AVI "))
}

// isMPEGTransportStream checks two sync bytes one packet apart. A single 0x47 is
// far too common to treat as a signature on its own.
func isMPEGTransportStream(header []byte) bool {
	return len(header) >= 189 && header[0] == 0x47 && header[188] == 0x47
}
