package shop

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	OzonCategorySearchIndexVersion = "ozon_path_ngram_v2"
	ozonCategorySearchIndexTTL     = 10 * time.Minute
	ozonCategorySearchMaxLimit     = 80
)

// OzonCategorySearchQuery is an internal, read-only query over the locally
// cached Ozon taxonomy. ProductType and SearchTerms are semantic expansions
// produced by the AI analysis pass; ProductTitle is a lower-weight recall lane.
type OzonCategorySearchQuery struct {
	ProductType      string
	SearchTerms      []string
	ProductTitle     string
	AllowedRootIDs   []string
	AllowedRootNames []string
	Limit            int
}

type OzonCategorySearchMatch struct {
	Node         OzonCategoryNodeDTO
	Score        float64
	MatchedTerms []string
	Lanes        []string
}

type OzonCategorySearchResult struct {
	Matches          []OzonCategorySearchMatch
	IndexVersion     string
	IndexedLeafCount int
	BuiltAt          time.Time
	CacheStale       bool
}

type ozonCategorySearchDocument struct {
	node       OzonCategoryNodeDTO
	leaf       string
	path       string
	segments   []string
	attributes string
	grams      map[string]struct{}
}

type ozonCategorySearchIndex struct {
	documents  []ozonCategorySearchDocument
	docFreq    map[string]int
	builtAt    time.Time
	cacheStale bool
}

type ozonCategoryWeightedTerm struct {
	raw        string
	normalized string
	weight     float64
	lane       string
}

// SearchOzonLeafCategories performs bounded hybrid recall without calling
// Ozon or an AI provider. The index covers every active cached leaf path and
// any locally cached template attribute names, and is rebuilt after taxonomy
// sync or a short TTL. AI remains responsible for semantic path review.
func (s *Service) SearchOzonLeafCategories(ctx context.Context, query OzonCategorySearchQuery) (*OzonCategorySearchResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	index, err := s.ozonLeafSearchIndex(ctx)
	if err != nil {
		return nil, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > ozonCategorySearchMaxLimit {
		limit = ozonCategorySearchMaxLimit
	}
	terms := ozonCategorySearchTerms(query)
	if len(terms) == 0 || len(index.documents) == 0 {
		return &OzonCategorySearchResult{
			Matches: []OzonCategorySearchMatch{}, IndexVersion: OzonCategorySearchIndexVersion,
			IndexedLeafCount: len(index.documents), BuiltAt: index.builtAt, CacheStale: index.cacheStale,
		}, nil
	}

	type scored struct {
		doc          ozonCategorySearchDocument
		score        float64
		matchedTerms []string
		lanes        []string
	}
	documents := filterOzonCategorySearchDocuments(index.documents, query.AllowedRootIDs, query.AllowedRootNames)
	runeFrequency := ozonCategorySearchLeafRuneFrequency(documents)
	ranked := make([]scored, 0, len(documents))
	for _, document := range documents {
		score, matchedTerms, lanes := scoreOzonCategorySearchDocument(document, index.docFreq, len(index.documents), terms)
		if len(query.AllowedRootIDs) > 0 || len(query.AllowedRootNames) > 0 {
			weakScore, weakTerms := scoreOzonCategoryRootWeakFallback(document, runeFrequency, len(documents), terms)
			if weakScore > score {
				score = weakScore
				matchedTerms = weakTerms
				lanes = []string{"root_weak"}
			}
		}
		if score <= 0 {
			continue
		}
		ranked = append(ranked, scored{doc: document, score: score, matchedTerms: matchedTerms, lanes: lanes})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].doc.node.Path != ranked[j].doc.node.Path {
			return ranked[i].doc.node.Path < ranked[j].doc.node.Path
		}
		return ranked[i].doc.node.CategoryID < ranked[j].doc.node.CategoryID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	matches := make([]OzonCategorySearchMatch, 0, len(ranked))
	for _, item := range ranked {
		matches = append(matches, OzonCategorySearchMatch{
			Node: item.doc.node, Score: roundOzonCategorySearchScore(item.score),
			MatchedTerms: item.matchedTerms, Lanes: item.lanes,
		})
	}
	return &OzonCategorySearchResult{
		Matches: matches, IndexVersion: OzonCategorySearchIndexVersion,
		IndexedLeafCount: len(index.documents), BuiltAt: index.builtAt, CacheStale: index.cacheStale,
	}, nil
}

func filterOzonCategorySearchDocuments(
	documents []ozonCategorySearchDocument,
	rootIDs []string,
	rootNames []string,
) []ozonCategorySearchDocument {
	allowedIDs := map[string]bool{}
	allowedNames := map[string]bool{}
	for _, value := range rootIDs {
		if value = strings.TrimSpace(value); value != "" {
			allowedIDs[value] = true
		}
	}
	for _, value := range rootNames {
		if value = normalizeOzonCategorySearchText(value); value != "" {
			allowedNames[value] = true
		}
	}
	if len(allowedIDs) == 0 && len(allowedNames) == 0 {
		return documents
	}
	filtered := make([]ozonCategorySearchDocument, 0, len(documents))
	for _, document := range documents {
		rootID := ""
		rootName := ""
		if len(document.node.Ancestors) > 0 {
			rootID = strings.TrimSpace(document.node.Ancestors[0].CategoryID)
			rootName = normalizeOzonCategorySearchText(document.node.Ancestors[0].Name)
		} else if segments := strings.Split(document.node.Path, "/"); len(segments) > 0 {
			rootName = normalizeOzonCategorySearchText(segments[0])
		}
		if allowedIDs[rootID] || allowedNames[rootName] {
			filtered = append(filtered, document)
		}
	}
	return filtered
}

func ozonCategorySearchLeafRuneFrequency(documents []ozonCategorySearchDocument) map[rune]int {
	frequency := map[rune]int{}
	for _, document := range documents {
		seen := map[rune]bool{}
		for _, value := range document.leaf {
			if !ozonCategorySearchWeakRune(value) || seen[value] {
				continue
			}
			seen[value] = true
			frequency[value]++
		}
	}
	return frequency
}

func scoreOzonCategoryRootWeakFallback(
	document ozonCategorySearchDocument,
	frequency map[rune]int,
	documentCount int,
	terms []ozonCategoryWeightedTerm,
) (float64, []string) {
	if documentCount == 0 {
		return 0, nil
	}
	documentRunes := map[rune]bool{}
	for _, value := range document.leaf {
		if ozonCategorySearchWeakRune(value) {
			documentRunes[value] = true
		}
	}
	if len(documentRunes) == 0 {
		return 0, nil
	}
	rawScore := 0.0
	matchedTerms := []string{}
	seenTerms := map[string]bool{}
	for _, term := range terms {
		termRunes := map[rune]bool{}
		for _, value := range term.normalized {
			if ozonCategorySearchWeakRune(value) {
				termRunes[value] = true
			}
		}
		matchedIDF := 0.0
		matchedCount := 0
		for value := range termRunes {
			if !documentRunes[value] {
				continue
			}
			idf := math.Log(1 + float64(documentCount+1)/float64(frequency[value]+1))
			if idf < .75 {
				continue
			}
			matchedIDF += idf
			matchedCount++
		}
		if matchedCount == 0 {
			continue
		}
		leafLength := utf8.RuneCountInString(strings.ReplaceAll(document.leaf, " ", ""))
		termLength := len(termRunes)
		denominator := leafLength
		if termLength < denominator {
			denominator = termLength
		}
		coverage := 1.0
		if denominator > 0 {
			coverage = float64(matchedCount) / float64(denominator)
		}
		rawScore += term.weight * matchedIDF * (.55 + coverage*.45)
		if !seenTerms[term.normalized] {
			matchedTerms = append(matchedTerms, truncateOzonCategorySearchString(term.raw, 80))
			seenTerms[term.normalized] = true
		}
	}
	if rawScore <= 0 {
		return 0, nil
	}
	score := (1 - math.Exp(-rawScore/3.5)) * .48
	if score > .48 {
		score = .48
	}
	if len(matchedTerms) > 8 {
		matchedTerms = matchedTerms[:8]
	}
	return score, matchedTerms
}

func ozonCategorySearchWeakRune(value rune) bool {
	return value > unicode.MaxASCII && (unicode.IsLetter(value) || unicode.IsDigit(value))
}

func (s *Service) ozonLeafSearchIndex(ctx context.Context) (*ozonCategorySearchIndex, error) {
	now := time.Now().UTC()
	s.ozonCategorySearchMu.RLock()
	cached := s.ozonCategorySearchIndex
	if cached != nil && now.Sub(cached.builtAt) < ozonCategorySearchIndexTTL {
		s.ozonCategorySearchMu.RUnlock()
		return cached, nil
	}
	s.ozonCategorySearchMu.RUnlock()

	s.ozonCategorySearchMu.Lock()
	defer s.ozonCategorySearchMu.Unlock()
	if cached = s.ozonCategorySearchIndex; cached != nil && now.Sub(cached.builtAt) < ozonCategorySearchIndexTTL {
		return cached, nil
	}
	index, err := s.buildOzonLeafSearchIndex(ctx, now)
	if err != nil {
		return nil, err
	}
	s.ozonCategorySearchIndex = index
	return index, nil
}

func (s *Service) buildOzonLeafSearchIndex(ctx context.Context, now time.Time) (*ozonCategorySearchIndex, error) {
	pool, err := s.ListOzonCategories(ctx, OzonCategoryListQuery{OnlyLeaf: true, ActiveOnly: true, AllMatches: true})
	if err != nil {
		return nil, err
	}
	attributeNames := map[string][]string{}
	var attrs []PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).
		Select("category_id", "name").
		Where("platform = ?", ozonPlatform).
		Find(&attrs).Error; err == nil {
		for _, attr := range attrs {
			name := normalizeOzonCategorySearchText(attr.Name)
			if name != "" {
				attributeNames[attr.CategoryID] = append(attributeNames[attr.CategoryID], name)
			}
		}
	}
	index := &ozonCategorySearchIndex{
		documents: make([]ozonCategorySearchDocument, 0, len(pool.List)),
		docFreq:   map[string]int{}, builtAt: now, cacheStale: pool.CacheStale,
	}
	for _, node := range pool.List {
		if !node.IsLeaf || node.Status != "active" || strings.TrimSpace(node.CategoryID) == "" {
			continue
		}
		leaf := normalizeOzonCategorySearchText(node.Name)
		path := normalizeOzonCategorySearchText(node.Path)
		if path == "" {
			path = leaf
			node.Path = strings.TrimSpace(node.Name)
		}
		segments := make([]string, 0, len(node.Ancestors)+1)
		for _, ancestor := range node.Ancestors {
			if value := normalizeOzonCategorySearchText(ancestor.Name); value != "" {
				segments = append(segments, value)
			}
		}
		if leaf != "" {
			segments = append(segments, leaf)
		}
		attributes := strings.Join(attributeNames[node.CategoryID], " ")
		grams := ozonCategorySearchNGrams(strings.Join([]string{leaf, path, attributes}, " "))
		for gram := range grams {
			index.docFreq[gram]++
		}
		index.documents = append(index.documents, ozonCategorySearchDocument{
			node: node, leaf: leaf, path: path, segments: segments,
			attributes: attributes, grams: grams,
		})
	}
	return index, nil
}

func (s *Service) invalidateOzonCategorySearchIndex() {
	if s == nil {
		return
	}
	s.ozonCategorySearchMu.Lock()
	s.ozonCategorySearchIndex = nil
	s.ozonCategorySearchMu.Unlock()
}

func ozonCategorySearchTerms(query OzonCategorySearchQuery) []ozonCategoryWeightedTerm {
	candidates := make([]ozonCategoryWeightedTerm, 0, len(query.SearchTerms)+2)
	candidates = append(candidates, ozonCategoryWeightedTerm{raw: query.ProductType, weight: 1, lane: "product_type"})
	for _, term := range query.SearchTerms {
		candidates = append(candidates, ozonCategoryWeightedTerm{raw: term, weight: .9, lane: "ai_term"})
	}
	candidates = append(candidates, ozonCategoryWeightedTerm{raw: query.ProductTitle, weight: .35, lane: "title"})
	seen := map[string]bool{}
	out := make([]ozonCategoryWeightedTerm, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.raw = strings.TrimSpace(candidate.raw)
		candidate.normalized = normalizeOzonCategorySearchText(candidate.raw)
		if utf8.RuneCountInString(candidate.normalized) < 2 || seen[candidate.normalized] {
			continue
		}
		seen[candidate.normalized] = true
		out = append(out, candidate)
	}
	return out
}

func scoreOzonCategorySearchDocument(
	document ozonCategorySearchDocument,
	docFreq map[string]int,
	documentCount int,
	terms []ozonCategoryWeightedTerm,
) (float64, []string, []string) {
	rawScore := 0.0
	matchedTerms := []string{}
	lanes := []string{}
	seenTerm := map[string]bool{}
	seenLane := map[string]bool{}
	for _, term := range terms {
		phraseScore := ozonCategoryPhraseScore(document, term.normalized)
		gramScore := ozonCategoryIDFGramScore(document.grams, ozonCategorySearchNGrams(term.normalized), docFreq, documentCount)
		score := term.weight * (phraseScore + gramScore*2.4)
		if score <= .08 {
			continue
		}
		rawScore += score
		if !seenTerm[term.normalized] {
			matchedTerms = append(matchedTerms, truncateOzonCategorySearchString(term.raw, 80))
			seenTerm[term.normalized] = true
		}
		if !seenLane[term.lane] {
			lanes = append(lanes, term.lane)
			seenLane[term.lane] = true
		}
	}
	if rawScore <= 0 {
		return 0, nil, nil
	}
	// Saturation keeps the score stable as AI emits additional synonyms.
	normalized := 1 - math.Exp(-rawScore/4.5)
	if normalized > 1 {
		normalized = 1
	}
	if len(matchedTerms) > 8 {
		matchedTerms = matchedTerms[:8]
	}
	return normalized, matchedTerms, lanes
}

func ozonCategoryPhraseScore(document ozonCategorySearchDocument, term string) float64 {
	if term == "" {
		return 0
	}
	if document.leaf == term {
		return 6
	}
	if strings.Contains(document.leaf, term) {
		return 4.2
	}
	if utf8.RuneCountInString(document.leaf) >= 2 && strings.Contains(term, document.leaf) {
		return 3.4
	}
	for index, segment := range document.segments {
		weight := 2.4
		if index == len(document.segments)-1 {
			weight = 3.2
		}
		if segment == term {
			return weight
		}
		if strings.Contains(segment, term) || (utf8.RuneCountInString(segment) >= 2 && strings.Contains(term, segment)) {
			return weight * .7
		}
	}
	if strings.Contains(document.attributes, term) {
		return .5
	}
	return 0
}

func ozonCategoryIDFGramScore(document, query map[string]struct{}, docFreq map[string]int, documentCount int) float64 {
	if len(query) == 0 || documentCount == 0 {
		return 0
	}
	matched := 0.0
	possible := 0.0
	for gram := range query {
		frequency := docFreq[gram]
		idf := math.Log(1 + float64(documentCount+1)/float64(frequency+1))
		possible += idf
		if _, ok := document[gram]; ok {
			matched += idf
		}
	}
	if possible == 0 {
		return 0
	}
	ratio := matched / possible
	if ratio < .12 {
		return 0
	}
	return ratio
}

func ozonCategorySearchNGrams(value string) map[string]struct{} {
	compact := []rune(strings.ReplaceAll(normalizeOzonCategorySearchText(value), " ", ""))
	out := map[string]struct{}{}
	for _, size := range []int{2, 3} {
		for index := 0; index+size <= len(compact); index++ {
			out[string(compact[index:index+size])] = struct{}{}
		}
	}
	return out
}

func normalizeOzonCategorySearchText(value string) string {
	var builder strings.Builder
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space && builder.Len() > 0 {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func truncateOzonCategorySearchString(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func roundOzonCategorySearchScore(value float64) float64 {
	return math.Round(value*10000) / 10000
}
