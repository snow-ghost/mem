package search

import (
	"sync"
)

// anchorTexts are short prototypical phrasings of each question type.
// At classify time we cosine-compare the query embedding to each
// type's mean anchor embedding and pick the highest-scoring type.
//
// The motivation: the heuristic ClassifyQuestion misses
// knowledge-update entirely (its phrasing is identical to
// single-session-user). An embedding model trained on broad text
// can still tell them apart by topical signal — questions about
// *how things change over time* cluster differently from one-off
// fact recall.
var anchorTexts = map[QuestionType][]string{
	TypeKnowledgeUpdate: {
		"What is the current value of X now, after recent updates?",
		"How many times have I done X up to now in total?",
		"What is my latest score / record / count for X?",
		"How often do I do X these days, compared to before?",
		"Where did X end up moving to or what is X's current status?",
	},
	TypeTemporalReasoning: {
		"When did X happen, and what happened first or last?",
		"How many days or weeks before / after did X occur?",
		"Which event came first, X or Y?",
		"What did I do earliest or latest in the timeline?",
	},
	TypeSingleSessionPreference: {
		"Can you recommend something for me based on my preferences?",
		"What would you suggest as a good fit for what I like?",
		"Please recommend a hotel, restaurant, or activity.",
	},
	TypeSingleSessionAssistant: {
		"Remind me about that thing you mentioned earlier.",
		"Going back to our previous chat, what did you suggest?",
		"What was the recommendation you made last time?",
	},
	TypeMultiSession: {
		"How many distinct projects, items, or activities have I dealt with overall?",
		"Across all our sessions combined, what is the total count?",
		"Sum up across our chats how many things of type X.",
	},
	TypeSingleSessionUser: {
		"What is some specific fact about me that I mentioned before?",
		"What did I tell you about a single event or detail?",
		"Where did I go, what did I redeem, what is my degree?",
	},
}

// AnchorClassifier classifies a query by cosine similarity between
// its embedding and per-type anchor embeddings. Anchor embeddings are
// computed once via the supplied embed func (any callable that takes
// text and returns its vector) and cached on the struct.
type AnchorClassifier struct {
	mu       sync.Mutex
	embed    func(text string) ([]float32, error)
	prepared bool
	anchors  map[QuestionType][]float32 // mean vector per type
}

// NewAnchorClassifier wraps an embed function. The function will be
// called once per anchor text on first Classify (lazy init).
func NewAnchorClassifier(embed func(string) ([]float32, error)) *AnchorClassifier {
	return &AnchorClassifier{embed: embed}
}

// Prepare embeds all anchor texts and computes the per-type mean
// vector. Idempotent and thread-safe.
func (c *AnchorClassifier) Prepare() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prepared {
		return nil
	}
	c.anchors = make(map[QuestionType][]float32, len(anchorTexts))
	for typ, texts := range anchorTexts {
		var sum []float32
		count := 0
		for _, t := range texts {
			v, err := c.embed(t)
			if err != nil {
				return err
			}
			if sum == nil {
				sum = make([]float32, len(v))
			}
			for i := range v {
				sum[i] += v[i]
			}
			count++
		}
		if count > 0 {
			for i := range sum {
				sum[i] /= float32(count)
			}
		}
		c.anchors[typ] = sum
	}
	c.prepared = true
	return nil
}

// Classify returns the question type whose anchor mean is most similar
// to the supplied query vector. Falls back to TypeSingleSessionUser if
// the classifier has no anchors prepared.
func (c *AnchorClassifier) Classify(qvec []float32) QuestionType {
	c.mu.Lock()
	prepared := c.prepared
	anchors := c.anchors
	c.mu.Unlock()
	if !prepared || len(anchors) == 0 || len(qvec) == 0 {
		return TypeSingleSessionUser
	}
	var bestType QuestionType
	bestSim := float32(-2)
	for typ, anc := range anchors {
		if len(anc) != len(qvec) {
			continue
		}
		sim := -cosineDist(qvec, anc) + 1 // back to similarity in [-1, 1]
		if sim > bestSim {
			bestSim = sim
			bestType = typ
		}
	}
	if bestType == "" {
		return TypeSingleSessionUser
	}
	return bestType
}
