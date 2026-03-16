package audio

import "math"

// Linear16ToMulaw converts 16-bit PCM samples to 8-bit mu-law.
func Linear16ToMulaw(pcmData []byte) []byte {
	numSamples := len(pcmData) / 2
	mulaw := make([]byte, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
		mulaw[i] = linearToMulaw(sample)
	}
	return mulaw
}

// Linear16ToAlaw converts 16-bit PCM samples to 8-bit A-law.
func Linear16ToAlaw(pcmData []byte) []byte {
	numSamples := len(pcmData) / 2
	alaw := make([]byte, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
		alaw[i] = linearToAlaw(sample)
	}
	return alaw
}

// linearToMulaw converts a 16-bit linear sample to 8-bit mu-law.
func linearToMulaw(sample int16) byte {
	const (
		mulawMax  = 0x1FFF
		mulawBias = 33
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

// ResampleLinear16 resamples 16-bit PCM from srcRate to dstRate using linear interpolation.
func ResampleLinear16(pcmData []byte, srcRate, dstRate int) []byte {
	if srcRate == dstRate {
		return pcmData
	}

	numSrcSamples := len(pcmData) / 2
	ratio := float64(srcRate) / float64(dstRate)
	numDstSamples := int(float64(numSrcSamples) / ratio)

	result := make([]byte, numDstSamples*2)
	for i := 0; i < numDstSamples; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		var sample float64
		s0 := int16(pcmData[srcIdx*2]) | int16(pcmData[srcIdx*2+1])<<8
		if srcIdx+1 < numSrcSamples {
			s1 := int16(pcmData[(srcIdx+1)*2]) | int16(pcmData[(srcIdx+1)*2+1])<<8
			sample = float64(s0)*(1-frac) + float64(s1)*frac
		} else {
			sample = float64(s0)
		}

		s := int16(sample)
		result[i*2] = byte(s)
		result[i*2+1] = byte(s >> 8)
	}
	return result
}
