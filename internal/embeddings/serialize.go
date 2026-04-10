package embeddings

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Encode packs a []float32 into a little-endian byte blob for SQLite storage.
// Layout: [4 bytes big-endian dim][dim * 4 bytes little-endian float32].
func Encode(vec []float32) []byte {
	if len(vec) == 0 {
		return nil
	}
	out := make([]byte, 4+4*len(vec))
	binary.BigEndian.PutUint32(out[:4], uint32(len(vec)))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(out[4+4*i:8+4*i], math.Float32bits(f))
	}
	return out
}

// Decode unpacks a blob produced by Encode.
func Decode(blob []byte) ([]float32, error) {
	if len(blob) < 4 {
		return nil, fmt.Errorf("blob too short")
	}
	dim := int(binary.BigEndian.Uint32(blob[:4]))
	if len(blob) != 4+4*dim {
		return nil, fmt.Errorf("blob size %d != 4+4*%d", len(blob), dim)
	}
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[4+4*i : 8+4*i]))
	}
	return vec, nil
}

// Cosine returns the cosine similarity of two equal-length vectors.
// Returns 0 if either is zero-length or norm is zero.
func Cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
