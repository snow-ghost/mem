package embeddings

import (
	"math"
	"testing"
)

func TestEncode_GivenVector_WhenDecoded_ThenRoundTrips(t *testing.T) {
	in := []float32{0.1, -0.5, 3.14159, 1e-6, -1e-6, 0, 1}
	blob := Encode(in)
	out, err := Decode(blob)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("len: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("pos %d: got %f want %f", i, out[i], in[i])
		}
	}
}

func TestEncode_GivenEmptyVector_WhenEncoded_ThenNil(t *testing.T) {
	if blob := Encode(nil); blob != nil {
		t.Errorf("got %v want nil", blob)
	}
}

func TestDecode_GivenShortBlob_WhenDecoded_ThenError(t *testing.T) {
	_, err := Decode([]byte{1, 2})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCosine_GivenIdenticalVectors_WhenCompared_ThenOne(t *testing.T) {
	v := []float32{1, 2, 3}
	got := Cosine(v, v)
	if math.Abs(float64(got-1)) > 1e-6 {
		t.Errorf("got %f want 1", got)
	}
}

func TestCosine_GivenOrthogonalVectors_WhenCompared_ThenZero(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	if got := Cosine(a, b); got != 0 {
		t.Errorf("got %f want 0", got)
	}
}

func TestCosine_GivenOppositeVectors_WhenCompared_ThenNegativeOne(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	if got := Cosine(a, b); math.Abs(float64(got+1)) > 1e-6 {
		t.Errorf("got %f want -1", got)
	}
}

func TestCosine_GivenMismatchedLength_WhenCompared_ThenZero(t *testing.T) {
	if got := Cosine([]float32{1, 2}, []float32{1, 2, 3}); got != 0 {
		t.Errorf("got %f want 0", got)
	}
}
