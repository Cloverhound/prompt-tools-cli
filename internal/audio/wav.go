package audio

import (
	"bytes"
	"encoding/binary"
)

// WAVHeader represents the canonical 44-byte WAV header.
type WAVHeader struct {
	ChunkID       [4]byte // "RIFF"
	ChunkSize     uint32  // file size - 8
	Format        [4]byte // "WAVE"
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1=PCM, 6=A-law, 7=mu-law
	NumChannels   uint16  // 1=mono
	SampleRate    uint32
	ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // NumChannels * BitsPerSample/8
	BitsPerSample uint16
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32  // audio data size
}

// WriteWAV wraps raw audio data with a WAV header.
func WriteWAV(audioData []byte, sampleRate int, encoding string) ([]byte, error) {
	bps := BitsPerSample(encoding)
	fmtCode := FormatCode(encoding)
	numChannels := uint16(1)
	byteRate := uint32(sampleRate) * uint32(numChannels) * uint32(bps) / 8
	blockAlign := numChannels * bps / 8
	dataSize := uint32(len(audioData))

	hdr := WAVHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize:     36 + dataSize,
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   fmtCode,
		NumChannels:   numChannels,
		SampleRate:    uint32(sampleRate),
		ByteRate:      byteRate,
		BlockAlign:    blockAlign,
		BitsPerSample: bps,
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: dataSize,
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, hdr); err != nil {
		return nil, err
	}
	buf.Write(audioData)
	return buf.Bytes(), nil
}
