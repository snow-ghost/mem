package search

import (
	"container/heap"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"sync"
)

// HNSWIndex is a minimal in-memory Hierarchical Navigable Small World
// index for cosine-similarity nearest-neighbour search. It is a pure-Go
// port of the algorithm from "Efficient and robust approximate nearest
// neighbor search using HNSW graphs" (Malkov & Yashunin 2018) with
// reasonable defaults baked in.
//
// Persistence: the index is in-memory only — the caller rebuilds it
// from the SQLite drawers.embedding column on startup. At ~100k
// drawers × 1024-dim float32 the index uses ~400MB plus ~M*log(N)
// graph edges; rebuild from disk is one full table scan + Insert per
// vector. Below ~5000 drawers full scan is faster than HNSW build,
// so the public Search* helpers keep the linear path as default.
type HNSWIndex struct {
	mu sync.RWMutex

	M              int
	MMax           int
	MMax0          int // max neighbours at layer 0
	EfConstruction int
	EfSearch       int
	mL             float64

	dim         int
	enterPoint  int
	maxLevel    int
	rng         *rand.Rand
	nodes       []*hnswNode
	idToInt     map[int64]int
	intToID     []int64
}

type hnswNode struct {
	vec       []float32
	neighbors [][]int // per-level neighbour lists
}

// NewHNSWIndex constructs an empty index with sensible defaults.
// dim is the embedding dimensionality; all inserted vectors must match.
func NewHNSWIndex(dim int) *HNSWIndex {
	return &HNSWIndex{
		M:              16,
		MMax:           16,
		MMax0:          32,
		EfConstruction: 200,
		EfSearch:       64,
		mL:             1.0 / math.Log(2.0),
		dim:            dim,
		enterPoint:     -1,
		maxLevel:       -1,
		rng:            rand.New(rand.NewSource(42)),
		idToInt:        make(map[int64]int),
	}
}

// Insert adds a vector with an external int64 id to the graph.
// Idempotent on the (id, vec) pair: re-inserting overwrites.
func (h *HNSWIndex) Insert(id int64, vec []float32) {
	if len(vec) != h.dim {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	// Pick the level for the new node.
	level := int(math.Floor(-math.Log(h.rng.Float64()) * h.mL))

	node := &hnswNode{
		vec:       vec,
		neighbors: make([][]int, level+1),
	}
	intID, exists := h.idToInt[id]
	if exists {
		h.nodes[intID] = node
	} else {
		intID = len(h.nodes)
		h.nodes = append(h.nodes, node)
		h.intToID = append(h.intToID, id)
		h.idToInt[id] = intID
	}

	// First node becomes the enter point.
	if h.enterPoint < 0 {
		h.enterPoint = intID
		h.maxLevel = level
		return
	}

	ep := h.enterPoint
	maxL := h.maxLevel

	// Greedy search down to level+1.
	for lc := maxL; lc > level; lc-- {
		w := h.searchLayer(vec, []int{ep}, 1, lc)
		ep = nearestFromCandidates(w, vec, h.nodes)
	}

	// Insert at each level from min(maxL, level) down to 0.
	startLevel := level
	if startLevel > maxL {
		startLevel = maxL
	}
	for lc := startLevel; lc >= 0; lc-- {
		w := h.searchLayer(vec, []int{ep}, h.EfConstruction, lc)
		neighbors := selectNeighborsHeuristic(vec, w, h.M, h.nodes)
		node.neighbors[lc] = append(node.neighbors[lc], neighbors...)
		// Add reverse edges + prune if over MMax.
		for _, n := range neighbors {
			h.nodes[n].neighbors[lc] = append(h.nodes[n].neighbors[lc], intID)
			mMax := h.MMax
			if lc == 0 {
				mMax = h.MMax0
			}
			if len(h.nodes[n].neighbors[lc]) > mMax {
				pruned := selectNeighborsHeuristic(h.nodes[n].vec, h.nodes[n].neighbors[lc], mMax, h.nodes)
				h.nodes[n].neighbors[lc] = pruned
			}
		}
		if len(w) > 0 {
			ep = w[0]
		}
	}

	if level > maxL {
		h.enterPoint = intID
		h.maxLevel = level
	}
}

// Search returns the top-k drawer IDs by cosine similarity to query.
// `ef` overrides EfSearch when > 0; pass 0 to use the index default.
func (h *HNSWIndex) Search(query []float32, k int, ef int) []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.enterPoint < 0 || len(query) != h.dim {
		return nil
	}
	if ef <= 0 {
		ef = h.EfSearch
	}
	if ef < k {
		ef = k
	}

	ep := h.enterPoint
	for lc := h.maxLevel; lc > 0; lc-- {
		w := h.searchLayer(query, []int{ep}, 1, lc)
		ep = nearestFromCandidates(w, query, h.nodes)
	}
	w := h.searchLayer(query, []int{ep}, ef, 0)

	// Pick top-k by distance to query (cosine = 1 - cosine sim, smaller = better).
	type scored struct {
		id   int
		dist float32
	}
	out := make([]scored, len(w))
	for i, n := range w {
		out[i] = scored{n, cosineDist(query, h.nodes[n].vec)}
	}
	// Partial sort
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].dist < out[i].dist {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > k {
		out = out[:k]
	}
	ids := make([]int64, len(out))
	for i, s := range out {
		ids[i] = h.intToID[s.id]
	}
	return ids
}

// Size returns the number of indexed vectors.
func (h *HNSWIndex) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// searchLayer (must hold lock). Returns up to `ef` candidate node ids.
func (h *HNSWIndex) searchLayer(query []float32, entryPoints []int, ef int, layer int) []int {
	visited := make(map[int]bool, ef*4)
	candidates := &nodeMinHeap{}
	results := &nodeMaxHeap{}
	heap.Init(candidates)
	heap.Init(results)

	for _, ep := range entryPoints {
		d := cosineDist(query, h.nodes[ep].vec)
		visited[ep] = true
		heap.Push(candidates, &heapItem{id: ep, dist: d})
		heap.Push(results, &heapItem{id: ep, dist: d})
	}

	for candidates.Len() > 0 {
		c := heap.Pop(candidates).(*heapItem)
		f := (*results)[0]
		if c.dist > f.dist && results.Len() >= ef {
			break
		}
		for _, n := range h.nodes[c.id].neighbors[layer] {
			if visited[n] {
				continue
			}
			visited[n] = true
			d := cosineDist(query, h.nodes[n].vec)
			worst := (*results)[0]
			if d < worst.dist || results.Len() < ef {
				heap.Push(candidates, &heapItem{id: n, dist: d})
				heap.Push(results, &heapItem{id: n, dist: d})
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	out := make([]int, results.Len())
	for i := range out {
		out[i] = (*results)[i].id
	}
	return out
}

// selectNeighborsHeuristic picks up to M neighbours, preferring closest
// to the candidate. Equivalent to algorithm 3 (simple heuristic) of the
// HNSW paper without the "extend candidates" / "keep pruned" extras —
// good enough for our recall budget.
func selectNeighborsHeuristic(q []float32, candidates []int, M int, nodes []*hnswNode) []int {
	if len(candidates) <= M {
		out := make([]int, len(candidates))
		copy(out, candidates)
		return out
	}
	type sd struct {
		id   int
		dist float32
	}
	scored := make([]sd, len(candidates))
	for i, c := range candidates {
		scored[i] = sd{c, cosineDist(q, nodes[c].vec)}
	}
	// Partial sort for top-M
	for i := 0; i < M; i++ {
		minIdx := i
		for j := i + 1; j < len(scored); j++ {
			if scored[j].dist < scored[minIdx].dist {
				minIdx = j
			}
		}
		scored[i], scored[minIdx] = scored[minIdx], scored[i]
	}
	out := make([]int, M)
	for i := 0; i < M; i++ {
		out[i] = scored[i].id
	}
	return out
}

func nearestFromCandidates(cands []int, q []float32, nodes []*hnswNode) int {
	if len(cands) == 0 {
		return -1
	}
	best := cands[0]
	bestD := cosineDist(q, nodes[best].vec)
	for _, c := range cands[1:] {
		d := cosineDist(q, nodes[c].vec)
		if d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// cosineDist = 1 - cosine_similarity, so smaller is closer.
func cosineDist(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 1
	}
	sim := dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
	return 1 - sim
}

// Marshal serializes the index into a compact binary blob suitable for
// storage in a SQLite BLOB column. Format:
//
//	magic(4)  = 'HNS1'
//	dim(4)    = embedding dim (uint32 BE)
//	count(4)  = #nodes (uint32 BE)
//	maxLevel(4)
//	enterPt(4)
//	for each node:
//	    intID(4) is implicit by position
//	    extID(8) int64 BE
//	    nLevels(2) uint16 BE
//	    for each level:
//	        nNeighbors(2) uint16 BE
//	        neighbors[ni] (4 bytes each, uint32 BE — internal IDs)
//	    vec[dim] float32 little-endian (matches embeddings.Encode tail)
func (h *HNSWIndex) Marshal() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.nodes) == 0 {
		return nil, errors.New("empty index")
	}
	// First pass to size the buffer
	size := 20 // header
	for _, n := range h.nodes {
		size += 8 + 2 // extID + nLevels
		for _, lvl := range n.neighbors {
			size += 2 + 4*len(lvl)
		}
		size += 4 * h.dim
	}
	buf := make([]byte, size)
	copy(buf[0:4], []byte("HNS1"))
	binary.BigEndian.PutUint32(buf[4:8], uint32(h.dim))
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(h.nodes)))
	binary.BigEndian.PutUint32(buf[12:16], uint32(h.maxLevel))
	binary.BigEndian.PutUint32(buf[16:20], uint32(h.enterPoint))
	off := 20
	for i, n := range h.nodes {
		extID := h.intToID[i]
		binary.BigEndian.PutUint64(buf[off:off+8], uint64(extID))
		off += 8
		binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(n.neighbors)))
		off += 2
		for _, lvl := range n.neighbors {
			binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(lvl)))
			off += 2
			for _, nb := range lvl {
				binary.BigEndian.PutUint32(buf[off:off+4], uint32(nb))
				off += 4
			}
		}
		for _, f := range n.vec {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(f))
			off += 4
		}
	}
	return buf, nil
}

// LoadHNSW reconstructs an index from a Marshal()-produced blob.
func LoadHNSW(buf []byte) (*HNSWIndex, error) {
	if len(buf) < 20 || string(buf[0:4]) != "HNS1" {
		return nil, errors.New("invalid HNSW blob")
	}
	dim := int(binary.BigEndian.Uint32(buf[4:8]))
	count := int(binary.BigEndian.Uint32(buf[8:12]))
	maxLevel := int(int32(binary.BigEndian.Uint32(buf[12:16])))
	ep := int(int32(binary.BigEndian.Uint32(buf[16:20])))
	idx := NewHNSWIndex(dim)
	idx.nodes = make([]*hnswNode, count)
	idx.intToID = make([]int64, count)
	idx.idToInt = make(map[int64]int, count)
	idx.maxLevel = maxLevel
	idx.enterPoint = ep
	off := 20
	for i := 0; i < count; i++ {
		if off+8+2 > len(buf) {
			return nil, errors.New("truncated blob")
		}
		extID := int64(binary.BigEndian.Uint64(buf[off : off+8]))
		off += 8
		nLevels := int(binary.BigEndian.Uint16(buf[off : off+2]))
		off += 2
		node := &hnswNode{neighbors: make([][]int, nLevels)}
		for l := 0; l < nLevels; l++ {
			if off+2 > len(buf) {
				return nil, errors.New("truncated blob (neighbors header)")
			}
			nNb := int(binary.BigEndian.Uint16(buf[off : off+2]))
			off += 2
			if off+4*nNb > len(buf) {
				return nil, errors.New("truncated blob (neighbors body)")
			}
			lvl := make([]int, nNb)
			for k := 0; k < nNb; k++ {
				lvl[k] = int(int32(binary.BigEndian.Uint32(buf[off : off+4])))
				off += 4
			}
			node.neighbors[l] = lvl
		}
		if off+4*dim > len(buf) {
			return nil, errors.New("truncated blob (vector)")
		}
		node.vec = make([]float32, dim)
		for k := 0; k < dim; k++ {
			node.vec[k] = math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
			off += 4
		}
		idx.nodes[i] = node
		idx.intToID[i] = extID
		idx.idToInt[extID] = i
	}
	return idx, nil
}

// Heap helpers for HNSW search ----------------------------------------

type heapItem struct {
	id   int
	dist float32
}

// nodeMinHeap orders by ascending distance (front = closest).
type nodeMinHeap []*heapItem

func (h nodeMinHeap) Len() int            { return len(h) }
func (h nodeMinHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h nodeMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeMinHeap) Push(x any)         { *h = append(*h, x.(*heapItem)) }
func (h *nodeMinHeap) Pop() any           { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

// nodeMaxHeap orders by descending distance (front = farthest).
type nodeMaxHeap []*heapItem

func (h nodeMaxHeap) Len() int            { return len(h) }
func (h nodeMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h nodeMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeMaxHeap) Push(x any)         { *h = append(*h, x.(*heapItem)) }
func (h *nodeMaxHeap) Pop() any           { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }
