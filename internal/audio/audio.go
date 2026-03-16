package audio

// Encoding constants matching Google Cloud TTS API values.
const (
	EncodingMulaw    = "mulaw"
	EncodingAlaw     = "alaw"
	EncodingLinear16 = "linear16"
	EncodingMP3      = "mp3"
)

// WAV format codes
const (
	WAVFormatPCM   = 1
	WAVFormatAlaw  = 6
	WAVFormatMulaw = 7
)

// FormatCode returns the WAV format code for an encoding.
func FormatCode(encoding string) uint16 {
	switch encoding {
	case EncodingAlaw:
		return WAVFormatAlaw
	case EncodingMulaw:
		return WAVFormatMulaw
	default:
		return WAVFormatPCM
	}
}

// BitsPerSample returns bits per sample for an encoding.
func BitsPerSample(encoding string) uint16 {
	switch encoding {
	case EncodingLinear16:
		return 16
	default:
		return 8 // mulaw and alaw are 8-bit
	}
}

// GoogleEncoding maps our encoding names to Google API encoding enum values.
func GoogleEncoding(encoding string) string {
	switch encoding {
	case EncodingMulaw:
		return "MULAW"
	case EncodingAlaw:
		return "ALAW"
	case EncodingLinear16:
		return "LINEAR16"
	case EncodingMP3:
		return "MP3"
	default:
		return "MULAW"
	}
}
