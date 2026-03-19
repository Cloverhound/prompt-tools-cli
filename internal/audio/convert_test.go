package audio

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- Test helpers ---

// generateSine creates a 16-bit PCM mono sine wave.
func generateSine(freq float64, sampleRate int, duration float64, amplitude float64) []byte {
	numSamples := int(float64(sampleRate) * duration)
	data := make([]byte, numSamples*2)
	for i := range numSamples {
		t := float64(i) / float64(sampleRate)
		s := int16(amplitude * math.Sin(2*math.Pi*freq*t))
		data[i*2] = byte(s)
		data[i*2+1] = byte(s >> 8)
	}
	return data
}

// generateDC creates a constant-value 16-bit PCM signal.
func generateDC(value int16, numSamples int) []byte {
	data := make([]byte, numSamples*2)
	for i := range numSamples {
		data[i*2] = byte(value)
		data[i*2+1] = byte(value >> 8)
	}
	return data
}

// rmsLevel computes RMS of 16-bit PCM data.
func rmsLevel(pcm []byte) float64 {
	n := len(pcm) / 2
	var sum float64
	for i := range n {
		s := float64(int16(pcm[i*2]) | int16(pcm[i*2+1])<<8)
		sum += s * s
	}
	return math.Sqrt(sum / float64(n))
}

// goertzelMagnitude returns the magnitude of a specific frequency in 16-bit PCM
// using the Goertzel algorithm. Result is normalized to amplitude.
func goertzelMagnitude(pcm []byte, sampleRate int, targetFreq float64) float64 {
	n := len(pcm) / 2
	k := int(math.Round(targetFreq * float64(n) / float64(sampleRate)))
	w := 2.0 * math.Pi * float64(k) / float64(n)
	coeff := 2.0 * math.Cos(w)

	var s0, s1, s2 float64
	for i := range n {
		sample := float64(int16(pcm[i*2]) | int16(pcm[i*2+1])<<8)
		s0 = sample + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}

	power := s1*s1 + s2*s2 - coeff*s1*s2
	return 2.0 * math.Sqrt(math.Abs(power)) / float64(n)
}

// mulawToLinear16 decodes 8-bit mu-law to 16-bit linear PCM.
func mulawToLinear16(mu byte) int16 {
	mu = ^mu
	sign := int16(1)
	if mu&0x80 != 0 {
		sign = -1
		mu &= 0x7F
	}
	exponent := int((mu >> 4) & 0x07)
	mantissa := int(mu & 0x0F)
	sample := ((mantissa << 3) + 0x84) << uint(exponent)
	sample -= 0x84
	return int16(sample) * sign
}

// alawToLinear16 decodes 8-bit A-law to 16-bit linear PCM.
func alawToLinear16(a byte) int16 {
	a ^= 0xD5
	sign := int16(1)
	if a&0x80 != 0 {
		sign = -1
		a &= 0x7F
	}
	exponent := int((a >> 4) & 0x07)
	mantissa := int(a & 0x0F)
	var sample int
	if exponent == 0 {
		sample = (mantissa << 4) + 8
	} else {
		sample = ((mantissa << 4) + 0x108) << uint(exponent-1)
	}
	return int16(sample) * sign
}

func hasSox() bool {
	_, err := exec.LookPath("sox")
	return err == nil
}

// --- Mu-law / A-law encoding tests ---

func TestLinearToMulawRoundtrip(t *testing.T) {
	testValues := []int16{0, 1, -1, 50, -50, 100, -100, 500, -500,
		1000, -1000, 5000, -5000, 10000, -10000, 20000, -20000, 32000, -32000}

	for _, val := range testValues {
		encoded := linearToMulaw(val)
		decoded := mulawToLinear16(encoded)
		err := math.Abs(float64(val) - float64(decoded))
		// Mu-law error: ~4% of magnitude for large values, bounded by segment 0 step (8) for small.
		maxErr := math.Max(math.Abs(float64(val))*0.05, 10)
		if err > maxErr {
			t.Errorf("mu-law roundtrip %d: decoded=%d, error=%.0f (max=%.0f)", val, decoded, err, maxErr)
		}
	}
}

func TestLinearToAlawRoundtrip(t *testing.T) {
	testValues := []int16{0, 1, -1, 50, -50, 100, -100, 500, -500,
		1000, -1000, 5000, -5000, 10000, -10000, 20000, -20000, 32000, -32000}

	for _, val := range testValues {
		encoded := linearToAlaw(val)
		decoded := alawToLinear16(encoded)
		err := math.Abs(float64(val) - float64(decoded))
		maxErr := math.Max(math.Abs(float64(val))*0.05, 10)
		if err > maxErr {
			t.Errorf("a-law roundtrip %d: decoded=%d, error=%.0f (max=%.0f)", val, decoded, err, maxErr)
		}
	}
}

func TestMulawSymmetry(t *testing.T) {
	// Positive and negative inputs of the same magnitude should produce
	// mu-law codes that differ only in the sign bit.
	for _, val := range []int16{100, 1000, 10000, 30000} {
		pos := linearToMulaw(val)
		neg := linearToMulaw(-val)
		if pos^0x80 != neg {
			t.Errorf("mu-law asymmetry at %d: +%02x -%02x", val, pos, neg)
		}
	}
}

func TestMulawMonotonicity(t *testing.T) {
	// Increasing input should produce non-decreasing decoded output.
	prev := mulawToLinear16(linearToMulaw(0))
	for val := int16(1); val < 32000; val += 100 {
		decoded := mulawToLinear16(linearToMulaw(val))
		if decoded < prev {
			t.Errorf("mu-law monotonicity violated: input %d decoded to %d, previous %d", val, decoded, prev)
		}
		prev = decoded
	}
}

// --- Dithering tests ---

func TestMulawDithering(t *testing.T) {
	// Low-amplitude signal where dithering has the most effect.
	pcm := generateSine(300, 8000, 0.1, 100)

	dithered := Linear16ToMulaw(pcm)

	// Direct encoding without dithering.
	n := len(pcm) / 2
	direct := make([]byte, n)
	for i := range n {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		direct[i] = linearToMulaw(s)
	}

	diffCount := 0
	for i := range n {
		if dithered[i] != direct[i] {
			diffCount++
		}
	}

	if diffCount == 0 {
		t.Error("TPDF dithering had no effect — all samples identical to non-dithered")
	}
	t.Logf("dithering changed %d/%d samples (%.1f%%)", diffCount, n, 100*float64(diffCount)/float64(n))
}

func TestAlawDithering(t *testing.T) {
	pcm := generateSine(300, 8000, 0.1, 100)

	dithered := Linear16ToAlaw(pcm)

	n := len(pcm) / 2
	direct := make([]byte, n)
	for i := range n {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		direct[i] = linearToAlaw(s)
	}

	diffCount := 0
	for i := range n {
		if dithered[i] != direct[i] {
			diffCount++
		}
	}

	if diffCount == 0 {
		t.Error("TPDF dithering had no effect on A-law")
	}
	t.Logf("dithering changed %d/%d samples (%.1f%%)", diffCount, n, 100*float64(diffCount)/float64(n))
}

// --- Resampling tests ---

func TestResamplePassthrough(t *testing.T) {
	pcm := generateSine(1000, 8000, 0.1, 16000)
	result := ResampleLinear16(pcm, 8000, 8000)
	if len(result) != len(pcm) {
		t.Fatalf("passthrough changed length: %d → %d", len(pcm), len(result))
	}
	for i := range pcm {
		if result[i] != pcm[i] {
			t.Fatalf("passthrough modified data at byte %d", i)
		}
	}
}

func TestResampleDCPreservation(t *testing.T) {
	pcm := generateDC(10000, 24000) // 1 second at 24kHz
	result := ResampleLinear16(pcm, 24000, 8000)

	n := len(result) / 2
	margin := 200 // skip filter edges
	for i := margin; i < n-margin; i++ {
		s := int16(result[i*2]) | int16(result[i*2+1])<<8
		err := math.Abs(float64(s) - 10000)
		if err > 50 {
			t.Fatalf("DC preservation failed at sample %d: got %d, want ~10000", i, s)
		}
	}
}

func TestResamplePreservesLowFrequency(t *testing.T) {
	amplitude := 16000.0
	pcm := generateSine(1000, 24000, 0.5, amplitude)
	result := ResampleLinear16(pcm, 24000, 8000)

	mag := goertzelMagnitude(result, 8000, 1000)
	ratio := mag / amplitude
	if ratio < 0.90 || ratio > 1.10 {
		t.Errorf("1kHz amplitude ratio after 24k→8k: %.3f (want 0.90–1.10)", ratio)
	}
	t.Logf("1kHz amplitude preservation: %.3f", ratio)
}

func TestResamplePreservesMultipleFrequencies(t *testing.T) {
	// Test several in-band frequencies to check for passband ripple.
	freqs := []float64{200, 500, 1000, 2000, 3000}
	amplitude := 16000.0

	for _, freq := range freqs {
		pcm := generateSine(freq, 24000, 0.5, amplitude)
		result := ResampleLinear16(pcm, 24000, 8000)
		mag := goertzelMagnitude(result, 8000, freq)
		ratio := mag / amplitude
		if ratio < 0.85 || ratio > 1.15 {
			t.Errorf("%.0fHz amplitude ratio: %.3f (want 0.85–1.15)", freq, ratio)
		}
		t.Logf("%.0fHz amplitude preservation: %.3f", freq, ratio)
	}
}

func TestResampleAttenuatesAboveNyquist(t *testing.T) {
	// 5kHz at 24kHz → 8kHz. Above 4kHz Nyquist — must be heavily attenuated.
	pcm := generateSine(5000, 24000, 0.5, 16000)
	result := ResampleLinear16(pcm, 24000, 8000)

	rms := rmsLevel(result)
	attenuation := 20 * math.Log10(rms/16000.0)
	if attenuation > -40 {
		t.Errorf("above-Nyquist attenuation: %.1f dB (want < -40 dB)", attenuation)
	}
	t.Logf("5kHz attenuation at 24k→8k: %.1f dB", attenuation)
}

func TestResampleSNR(t *testing.T) {
	freq := 1000.0
	amplitude := 16000.0
	pcm := generateSine(freq, 24000, 0.5, amplitude)
	result := ResampleLinear16(pcm, 24000, 8000)

	n := len(result) / 2
	margin := 200
	var signalPower, noisePower float64
	for i := margin; i < n-margin; i++ {
		actual := float64(int16(result[i*2]) | int16(result[i*2+1])<<8)
		tSec := float64(i) / 8000.0
		expected := amplitude * math.Sin(2*math.Pi*freq*tSec)
		signalPower += expected * expected
		diff := actual - expected
		noisePower += diff * diff
	}

	snr := 10 * math.Log10(signalPower/noisePower)
	if snr < 60 {
		t.Errorf("resample SNR: %.1f dB (want ≥ 60 dB)", snr)
	}
	t.Logf("24kHz→8kHz resample SNR for 1kHz sine: %.1f dB", snr)
}

func TestResample48kTo8k(t *testing.T) {
	// Higher ratio: 48kHz → 8kHz (6:1).
	freq := 1000.0
	amplitude := 16000.0
	pcm := generateSine(freq, 48000, 0.5, amplitude)
	result := ResampleLinear16(pcm, 48000, 8000)

	mag := goertzelMagnitude(result, 8000, freq)
	ratio := mag / amplitude
	if ratio < 0.85 || ratio > 1.15 {
		t.Errorf("1kHz amplitude ratio at 48k→8k: %.3f", ratio)
	}

	n := len(result) / 2
	margin := 300
	var signalPower, noisePower float64
	for i := margin; i < n-margin; i++ {
		actual := float64(int16(result[i*2]) | int16(result[i*2+1])<<8)
		tSec := float64(i) / 8000.0
		expected := amplitude * math.Sin(2*math.Pi*freq*tSec)
		signalPower += expected * expected
		diff := actual - expected
		noisePower += diff * diff
	}

	snr := 10 * math.Log10(signalPower/noisePower)
	if snr < 50 {
		t.Errorf("48k→8k resample SNR: %.1f dB (want ≥ 50 dB)", snr)
	}
	t.Logf("48kHz→8kHz resample SNR: %.1f dB, amplitude: %.3f", snr, ratio)
}

// --- Sox comparison tests ---

func TestResampleVsSox(t *testing.T) {
	if !hasSox() {
		t.Skip("sox not installed")
	}

	pcm := generateSine(1000, 24000, 1.0, 16000)

	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "input.raw")
	soxOutFile := filepath.Join(tmpDir, "sox_output.raw")

	if err := os.WriteFile(inFile, pcm, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sox",
		"-t", "raw", "-r", "24000", "-e", "signed-integer", "-b", "16", "-c", "1", "-L", inFile,
		"-t", "raw", "-r", "8000", "-e", "signed-integer", "-b", "16", "-c", "1", "-L", soxOutFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sox failed: %v\n%s", err, out)
	}

	soxData, _ := os.ReadFile(soxOutFile)
	ourData := ResampleLinear16(pcm, 24000, 8000)

	soxSamples := len(soxData) / 2
	ourSamples := len(ourData) / 2
	minSamples := soxSamples
	if ourSamples < minSamples {
		minSamples = ourSamples
	}

	margin := 200
	var diffPower, soxPower float64
	for i := margin; i < minSamples-margin; i++ {
		soxS := float64(int16(soxData[i*2]) | int16(soxData[i*2+1])<<8)
		ourS := float64(int16(ourData[i*2]) | int16(ourData[i*2+1])<<8)
		diff := ourS - soxS
		diffPower += diff * diff
		soxPower += soxS * soxS
	}

	snrVsSox := 10 * math.Log10(soxPower/diffPower)
	t.Logf("resample SNR vs sox: %.1f dB (ours: %d samples, sox: %d samples)", snrVsSox, ourSamples, soxSamples)

	if snrVsSox < 40 {
		t.Errorf("resample vs sox SNR: %.1f dB (want ≥ 40 dB)", snrVsSox)
	}
}

func TestFullPipelineVsSox(t *testing.T) {
	if !hasSox() {
		t.Skip("sox not installed")
	}

	pcm := generateSine(1000, 24000, 1.0, 16000)

	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "input.raw")
	soxOutFile := filepath.Join(tmpDir, "sox_output.raw")

	if err := os.WriteFile(inFile, pcm, 0644); err != nil {
		t.Fatal(err)
	}

	// Sox: resample + mu-law encode in one step.
	cmd := exec.Command("sox",
		"-t", "raw", "-r", "24000", "-e", "signed-integer", "-b", "16", "-c", "1", "-L", inFile,
		"-t", "raw", "-r", "8000", "-e", "mu-law", "-b", "8", "-c", "1", soxOutFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sox failed: %v\n%s", err, out)
	}

	soxMulaw, _ := os.ReadFile(soxOutFile)

	// Our pipeline: resample → mu-law.
	resampled := ResampleLinear16(pcm, 24000, 8000)
	ourMulaw := Linear16ToMulaw(resampled)

	minLen := len(soxMulaw)
	if len(ourMulaw) < minLen {
		minLen = len(ourMulaw)
	}

	// Decode both back to linear and compare.
	margin := 200
	var diffPower, sigPower float64
	for i := margin; i < minLen-margin; i++ {
		soxLin := float64(mulawToLinear16(soxMulaw[i]))
		ourLin := float64(mulawToLinear16(ourMulaw[i]))
		diff := ourLin - soxLin
		diffPower += diff * diff
		sigPower += soxLin * soxLin
	}

	snr := 10 * math.Log10(sigPower/diffPower)
	t.Logf("full pipeline (24k→8k mu-law) SNR vs sox: %.1f dB (ours: %d, sox: %d samples)",
		snr, len(ourMulaw), len(soxMulaw))

	if snr < 30 {
		t.Errorf("full pipeline vs sox SNR: %.1f dB (want ≥ 30 dB)", snr)
	}
}

func TestFullPipeline48kVsSox(t *testing.T) {
	if !hasSox() {
		t.Skip("sox not installed")
	}

	pcm := generateSine(1000, 48000, 1.0, 16000)

	tmpDir := t.TempDir()
	inFile := filepath.Join(tmpDir, "input.raw")
	soxOutFile := filepath.Join(tmpDir, "sox_output.raw")

	if err := os.WriteFile(inFile, pcm, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sox",
		"-t", "raw", "-r", "48000", "-e", "signed-integer", "-b", "16", "-c", "1", "-L", inFile,
		"-t", "raw", "-r", "8000", "-e", "mu-law", "-b", "8", "-c", "1", soxOutFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sox failed: %v\n%s", err, out)
	}

	soxMulaw, _ := os.ReadFile(soxOutFile)

	resampled := ResampleLinear16(pcm, 48000, 8000)
	ourMulaw := Linear16ToMulaw(resampled)

	minLen := len(soxMulaw)
	if len(ourMulaw) < minLen {
		minLen = len(ourMulaw)
	}

	margin := 300
	var diffPower, sigPower float64
	for i := margin; i < minLen-margin; i++ {
		soxLin := float64(mulawToLinear16(soxMulaw[i]))
		ourLin := float64(mulawToLinear16(ourMulaw[i]))
		diff := ourLin - soxLin
		diffPower += diff * diff
		sigPower += soxLin * soxLin
	}

	snr := 10 * math.Log10(sigPower/diffPower)
	t.Logf("full pipeline (48k→8k mu-law) SNR vs sox: %.1f dB (ours: %d, sox: %d samples)",
		snr, len(ourMulaw), len(soxMulaw))

	if snr < 30 {
		t.Errorf("48k pipeline vs sox SNR: %.1f dB (want ≥ 30 dB)", snr)
	}
}
