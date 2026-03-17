package provider

// Voice represents a TTS voice.
type Voice struct {
	Name              string   `json:"name"`
	VoiceID           string   `json:"voice_id,omitempty"`
	Model             string   `json:"model"`
	LanguageCodes     []string `json:"language_codes"`
	Gender            string   `json:"gender"`
	Provider          string   `json:"provider"`
	NaturalSampleRate int      `json:"natural_sample_rate,omitempty"`
}

// TTSRequest contains parameters for speech synthesis.
type TTSRequest struct {
	Text         string
	SSML         string // if non-empty, used instead of Text
	Voice        string
	Model        string // Gemini model name (e.g., gemini-2.5-pro-tts); auto-set for Gemini voices if empty
	SampleRate   int
	Encoding     string // mulaw, alaw, linear16, mp3
	SpeakingRate float64
	Pitch        float64
	VolumeGainDb float64
}

// TTSResult contains the synthesized audio.
type TTSResult struct {
	AudioData  []byte
	Format     string // the encoding format of AudioData
	SampleRate int
}

// TTSProvider generates speech from text.
type TTSProvider interface {
	Name() string
	Synthesize(req *TTSRequest) (*TTSResult, error)
	ListVoices(languageCode string) ([]Voice, error)
}

// STTRequest contains parameters for speech-to-text.
type STTRequest struct {
	AudioData    []byte
	AudioFile    string // file path (alternative to AudioData)
	LanguageCode string
	Timestamps   bool
	Phrases      []string // boost phrases
}

// WordTimestamp represents a word with timing info.
type WordTimestamp struct {
	Word      string  `json:"word"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
}

// TranscriptionResult contains the transcribed text.
type TranscriptionResult struct {
	Text       string          `json:"text"`
	Words      []WordTimestamp  `json:"words,omitempty"`
	Confidence float64         `json:"confidence,omitempty"`
}

// STTProvider transcribes audio to text.
type STTProvider interface {
	Name() string
	Transcribe(req *STTRequest) (*TranscriptionResult, error)
}
