package search

import (
	"math/rand"
	"sort"
	"testing"
)

func randVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	return v
}

func TestHNSW_GivenSmallSet_WhenSearched_ThenReturnsKnownNearest(t *testing.T) {
	idx := NewHNSWIndex(3)
	idx.Insert(1, []float32{1, 0, 0})
	idx.Insert(2, []float32{0, 1, 0})
	idx.Insert(3, []float32{0, 0, 1})
	idx.Insert(4, []float32{1, 1, 0})

	got := idx.Search([]float32{0.9, 0.1, 0}, 1, 0)
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("got %v, want [1]", got)
	}
}

func TestHNSW_GivenManyRandomVecs_WhenSearched_ThenRecallAcceptable(t *testing.T) {
	const N = 1000
	const dim = 64
	const k = 10
	rng := rand.New(rand.NewSource(7))

	vectors := make([][]float32, N)
	idx := NewHNSWIndex(dim)
	for i := 0; i < N; i++ {
		vectors[i] = randVec(rng, dim)
		idx.Insert(int64(i+1), vectors[i])
	}

	// 50 random queries; measure recall@k against brute-force ground truth.
	totalRecall := 0.0
	const queries = 50
	for q := 0; q < queries; q++ {
		query := randVec(rng, dim)

		// Ground truth: top-k by cosine sim
		type sd struct {
			id   int64
			dist float32
		}
		gt := make([]sd, N)
		for i, v := range vectors {
			gt[i] = sd{int64(i + 1), cosineDist(query, v)}
		}
		sort.Slice(gt, func(i, j int) bool { return gt[i].dist < gt[j].dist })
		gtSet := make(map[int64]bool, k)
		for i := 0; i < k; i++ {
			gtSet[gt[i].id] = true
		}

		// HNSW
		got := idx.Search(query, k, 0)
		hits := 0
		for _, id := range got {
			if gtSet[id] {
				hits++
			}
		}
		totalRecall += float64(hits) / float64(k)
	}
	avgRecall := totalRecall / float64(queries)
	if avgRecall < 0.85 {
		t.Errorf("avg recall@%d = %.3f, want >= 0.85 (1k random vecs, dim=%d)", k, avgRecall, dim)
	}
	t.Logf("HNSW recall@%d on %d random %d-d vecs: %.3f", k, N, dim, avgRecall)
}

func TestHNSW_GivenEmptyIndex_WhenSearched_ThenEmpty(t *testing.T) {
	idx := NewHNSWIndex(8)
	if got := idx.Search([]float32{1, 0, 0, 0, 0, 0, 0, 0}, 5, 0); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

// fullScan is the baseline brute-force we measure HNSW against.
func fullScan(query []float32, vecs [][]float32, ids []int64, k int) []int64 {
	type sd struct {
		id   int64
		dist float32
	}
	out := make([]sd, len(vecs))
	for i, v := range vecs {
		out[i] = sd{ids[i], cosineDist(query, v)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dist < out[j].dist })
	if len(out) > k {
		out = out[:k]
	}
	res := make([]int64, len(out))
	for i, s := range out {
		res[i] = s.id
	}
	return res
}

func benchHNSWvsScan(b *testing.B, n, dim int) {
	rng := rand.New(rand.NewSource(7))
	vecs := make([][]float32, n)
	ids := make([]int64, n)
	idx := NewHNSWIndex(dim)
	for i := 0; i < n; i++ {
		vecs[i] = randVec(rng, dim)
		ids[i] = int64(i + 1)
		idx.Insert(ids[i], vecs[i])
	}
	queries := make([][]float32, 100)
	for i := range queries {
		queries[i] = randVec(rng, dim)
	}
	b.Run("HNSW", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Search(queries[i%len(queries)], 10, 0)
		}
	})
	b.Run("FullScan", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fullScan(queries[i%len(queries)], vecs, ids, 10)
		}
	})
}

func BenchmarkHNSW_1k_dim128(b *testing.B)   { benchHNSWvsScan(b, 1000, 128) }
func BenchmarkHNSW_10k_dim128(b *testing.B)  { benchHNSWvsScan(b, 10000, 128) }
func BenchmarkHNSW_50k_dim128(b *testing.B)  { benchHNSWvsScan(b, 50000, 128) }
