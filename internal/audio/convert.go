package audio

import (
	"math"
	"math/rand/v2"
)

// Linear16ToMulaw converts 16-bit PCM samples to 8-bit mu-law.
// Applies TPDF dithering to decorrelate quantization error.
func Linear16ToMulaw(pcmData []byte) []byte {
	numSamples := len(pcmData) / 2
	mulaw := make([]byte, numSamples)
	rng := rand.New(rand.NewPCG(0, 42))
	for i := 0; i < numSamples; i++ {
		sample := int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
		// TPDF dithering: triangular noise scaled to mu-law's minimum step size (2 in 16-bit).
		// Sum of two uniform[-1,1] values gives triangular[-2,2] distribution.
		dither := (rng.Float64()*2 - 1) + (rng.Float64()*2 - 1)
		dithered := float64(sample) + dither
		if dithered > 32767 {
			dithered = 32767
		} else if dithered < -32768 {
			dithered = -32768
		}
		mulaw[i] = linearToMulaw(int16(math.Round(dithered)))
	}
	return mulaw
}

// Linear16ToAlaw converts 16-bit PCM samples to 8-bit A-law.
// Applies TPDF dithering to decorrelate quantization error.
func Linear16ToAlaw(pcmData []byte) []byte {
	numSamples := len(pcmData) / 2
	alaw := make([]byte, numSamples)
	rng := rand.New(rand.NewPCG(0, 42))
	for i := 0; i < numSamples; i++ {
		sample := int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
		dither := (rng.Float64()*2 - 1) + (rng.Float64()*2 - 1)
		dithered := float64(sample) + dither
		if dithered > 32767 {
			dithered = 32767
		} else if dithered < -32768 {
			dithered = -32768
		}
		alaw[i] = linearToAlaw(int16(math.Round(dithered)))
	}
	return alaw
}

// linearToMulaw converts a 16-bit linear sample to 8-bit mu-law.
func linearToMulaw(sample int16) byte {
	const (
		mulawBias = 0x84 // 132, per ITU G.711
		mulawClip = 32635
	)

	sign := byte(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}
	if sample > mulawClip {
		sample = mulawClip
	}
	sample += mulawBias

	exponent := 7
	for expMask := int16(0x4000); (sample&expMask) == 0 && exponent > 0; expMask >>= 1 {
		exponent--
	}
	mantissa := (sample >> (uint(exponent) + 3)) & 0x0F

	return ^(sign | byte(exponent<<4) | byte(mantissa))
}

// linearToAlaw converts a 16-bit linear sample to 8-bit A-law.
func linearToAlaw(sample int16) byte {
	sign := 0
	if sample >= 0 {
		sign = 0xD5
	} else {
		sample = -sample - 1
		sign = 0x55
	}

	if sample > 32767 {
		sample = 32767
	}

	var exponent int
	var mantissa int

	absVal := int(sample)
	if absVal < 256 {
		exponent = 0
		mantissa = absVal >> 4
	} else {
		exponent = int(math.Log2(float64(absVal))) - 7
		if exponent < 0 {
			exponent = 0
		}
		if exponent > 7 {
			exponent = 7
		}
		mantissa = (absVal >> (uint(exponent) + 3)) & 0x0F
	}

	return byte((exponent << 4) | mantissa) ^ byte(sign)
}

// ResampleLinear16 resamples 16-bit PCM from srcRate to dstRate using
// polyphase windowed-sinc interpolation. This combines anti-alias filtering
// and interpolation into a single step for maximum quality.
func ResampleLinear16(pcmData []byte, srcRate, dstRate int) []byte {
	if srcRate == dstRate {
		return pcmData
	}

	numSrcSamples := len(pcmData) / 2
	src := make([]float64, numSrcSamples)
	for i := 0; i < numSrcSamples; i++ {
		src[i] = float64(int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8)
	}

	ratio := float64(srcRate) / float64(dstRate)
	numDstSamples := int(float64(numSrcSamples) / ratio)

	// Normalized cutoff frequency (relative to source sample rate).
	// Use 0.95 * Nyquist to give the transition band room to attenuate
	// before Nyquist, preventing aliasing energy from leaking through.
	var fc float64
	if dstRate < srcRate {
		// Downsampling: cut at 0.95 * target Nyquist
		fc = 0.95 * float64(dstRate) / (2.0 * float64(srcRate))
	} else {
		// Upsampling: cut at 0.95 * source Nyquist (anti-imaging)
		fc = 0.95 * 0.5
	}

	// Kernel half-length sized by zero-crossings of the sinc function.
	// One zero-crossing occurs every 1/(2*fc) input samples.
	// 32 zero-crossings per side gives excellent stopband attenuation.
	const numZeroCrossings = 32
	halfLen := int(math.Ceil(float64(numZeroCrossings) / (2.0 * fc)))
	if halfLen < 32 {
		halfLen = 32
	}
	if halfLen > 512 {
		halfLen = 512
	}

	result := make([]byte, numDstSamples*2)
	for i := 0; i < numDstSamples; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(math.Floor(srcPos))

		jMin := srcIdx - halfLen + 1
		if jMin < 0 {
			jMin = 0
		}
		jMax := srcIdx + halfLen
		if jMax >= numSrcSamples {
			jMax = numSrcSamples - 1
		}

		var sample float64
		var weightSum float64
		for j := jMin; j <= jMax; j++ {
			d := float64(j) - srcPos

			// Windowed sinc: sinc(2*fc*d) * blackman(d)
			var sincVal float64
			arg := 2.0 * fc * d
			if math.Abs(arg) < 1e-9 {
				sincVal = 1.0
			} else {
				sincVal = math.Sin(math.Pi*arg) / (math.Pi * arg)
			}

			// Blackman window centered at srcPos, spanning ±halfLen
			wPos := (d + float64(halfLen)) / float64(2*halfLen)
			w := 0.42 - 0.5*math.Cos(2*math.Pi*wPos) + 0.08*math.Cos(4*math.Pi*wPos)

			weight := sincVal * w
			sample += src[j] * weight
			weightSum += weight
		}

		// Normalize to preserve amplitude
		if weightSum != 0 {
			sample /= weightSum
		}

		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}

		s := int16(sample)
		result[i*2] = byte(s)
		result[i*2+1] = byte(s >> 8)
	}
	return result
}
