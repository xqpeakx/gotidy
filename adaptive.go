package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	learningManifestName    = ".gotidy-learning.json"
	learningManifestVersion = 1
	maxContentHintRead      = 16 * 1024
)

type CategoryDecision struct {
	Rule       CategoryRule `json:"rule"`
	Source     string       `json:"source"`
	Reason     string       `json:"reason,omitempty"`
	Confidence float64      `json:"confidence,omitempty"`
}

type Categorizer struct {
	Root         string
	Resolver     CategoryResolver
	Adaptive     bool
	Learn        bool
	ContentHints bool
	Learning     *LearningStore
	profile      *DirectoryProfile
}

type LearningStore struct {
	Path    string
	Data    learningManifest
	Exists  bool
	updated bool
}

type learningManifest struct {
	Version    int                             `json:"version"`
	UpdatedAt  string                          `json:"updated_at,omitempty"`
	Extensions map[string][]learningPreference `json:"extensions,omitempty"`
	Tokens     map[string][]learningPreference `json:"tokens,omitempty"`
}

type learningPreference struct {
	Category    string `json:"category"`
	Destination string `json:"destination"`
	Count       int    `json:"count"`
}

type DirectoryProfile struct {
	ByExtension map[string]map[string]int
	ByStem      map[string]map[string]int
	ByToken     map[string]map[string]int
}

func NewCategorizer(root string, resolver CategoryResolver, adaptive, learn, contentHints bool) (*Categorizer, error) {
	if root == "" {
		root = "."
	}

	c := &Categorizer{
		Root:         root,
		Resolver:     resolver,
		Adaptive:     adaptive || learn,
		Learn:        learn,
		ContentHints: contentHints,
	}

	if c.Adaptive || c.Learn {
		learning, err := loadLearningStore(root)
		if err != nil {
			return nil, err
		}
		c.Learning = learning
	}

	return c, nil
}

func (c *Categorizer) Classify(name string) (CategoryDecision, error) {
	baseRule := c.Resolver.Resolve(name)
	decision := CategoryDecision{
		Rule:       baseRule,
		Source:     baseRule.Origin,
		Confidence: baseConfidenceForRule(name, baseRule),
	}

	if !c.Adaptive {
		return decision, nil
	}
	if baseRule.Origin == "config" {
		decision.Source = "config"
		decision.Confidence = 1
		return decision, nil
	}

	if candidate, ok := c.learnedByExtension(name); ok {
		return candidate, nil
	}
	if candidate, ok := c.learnedByTokens(name); ok {
		return candidate, nil
	}
	if candidate, ok, err := c.profileByStem(name); err != nil {
		return decision, err
	} else if ok {
		return candidate, nil
	}
	if candidate, ok := c.profileByTokens(name); ok {
		return candidate, nil
	}
	if candidate, ok := c.profileByExtension(name, baseRule); ok {
		return candidate, nil
	}
	if c.ContentHints {
		if candidate, ok, err := c.classifyByContent(name, decision); err != nil {
			return decision, err
		} else if ok {
			return candidate, nil
		}
	}

	return decision, nil
}

func (c *Categorizer) Observe(name string, rule CategoryRule) {
	if !c.Learn || c.Learning == nil {
		return
	}
	c.Learning.Observe(name, rule)
}

func (c *Categorizer) Save() error {
	if !c.Learn || c.Learning == nil {
		return nil
	}
	return c.Learning.Save()
}

func (c *Categorizer) LearningPath() string {
	if c.Learning == nil || (!c.Learning.Exists && !c.Learning.updated) {
		return ""
	}
	return c.Learning.Path
}

func (c *Categorizer) LearningDefinitions() []CategoryDefinition {
	if c.Learning == nil {
		return nil
	}

	prefs := c.Learning.StrongExtensionPreferences()
	definitions := make([]CategoryDefinition, 0, len(prefs))
	for _, preference := range prefs {
		definitions = append(definitions, CategoryDefinition{
			Name:        preference.Category,
			Destination: preference.Destination,
			Extensions:  []string{preference.Extension},
		})
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Name != definitions[j].Name {
			return definitions[i].Name < definitions[j].Name
		}
		return definitions[i].Extensions[0] < definitions[j].Extensions[0]
	})
	return definitions
}

func (c *Categorizer) learnedByExtension(name string) (CategoryDecision, bool) {
	if c.Learning == nil {
		return CategoryDecision{}, false
	}

	ext := normalizeExtension(filepath.Ext(name))
	if ext == "" {
		return CategoryDecision{}, false
	}

	preference, confidence, ok := c.Learning.BestExtension(ext)
	if !ok {
		return CategoryDecision{}, false
	}

	return CategoryDecision{
		Rule:       CategoryRule{Name: preference.Category, Destination: preference.Destination, Origin: "learned"},
		Source:     "learned-extension",
		Reason:     fmt.Sprintf("learned .%s -> %s from prior runs", ext, preference.Destination),
		Confidence: confidence,
	}, true
}

func (c *Categorizer) learnedByTokens(name string) (CategoryDecision, bool) {
	if c.Learning == nil {
		return CategoryDecision{}, false
	}

	preference, matchedToken, confidence, ok := c.Learning.BestTokens(name)
	if !ok {
		return CategoryDecision{}, false
	}

	return CategoryDecision{
		Rule:       CategoryRule{Name: preference.Category, Destination: preference.Destination, Origin: "learned"},
		Source:     "learned-token",
		Reason:     fmt.Sprintf("learned token %q -> %s from prior runs", matchedToken, preference.Destination),
		Confidence: confidence,
	}, true
}

func (c *Categorizer) profileByStem(name string) (CategoryDecision, bool, error) {
	profile, err := c.directoryProfile()
	if err != nil {
		return CategoryDecision{}, false, err
	}

	stem := normalizedStem(name)
	if stem == "" {
		return CategoryDecision{}, false, nil
	}

	rule, confidence, ok := topRule(profile.ByStem[stem], 1, 0.6)
	if !ok {
		return CategoryDecision{}, false, nil
	}

	return CategoryDecision{
		Rule:       rule,
		Source:     "heuristic-stem",
		Reason:     fmt.Sprintf("matched related files named %q in %s", stem, rule.Destination),
		Confidence: confidence,
	}, true, nil
}

func (c *Categorizer) profileByTokens(name string) (CategoryDecision, bool) {
	profile, err := c.directoryProfile()
	if err != nil {
		return CategoryDecision{}, false
	}

	scores := make(map[string]int)
	matchedToken := ""
	for _, token := range tokenizeName(name) {
		tokenScores := profile.ByToken[token]
		if len(tokenScores) == 0 {
			continue
		}
		if matchedToken == "" {
			matchedToken = token
		}
		for key, score := range tokenScores {
			scores[key] += score
		}
	}

	rule, confidence, ok := topRule(scores, 2, 0.65)
	if !ok {
		return CategoryDecision{}, false
	}

	return CategoryDecision{
		Rule:       rule,
		Source:     "heuristic-token",
		Reason:     fmt.Sprintf("matched related filenames using token %q", matchedToken),
		Confidence: confidence,
	}, true
}

func (c *Categorizer) profileByExtension(name string, baseRule CategoryRule) (CategoryDecision, bool) {
	profile, err := c.directoryProfile()
	if err != nil {
		return CategoryDecision{}, false
	}

	ext := normalizeExtension(filepath.Ext(name))
	if ext == "" {
		return CategoryDecision{}, false
	}

	rule, confidence, ok := topRule(profile.ByExtension[ext], 2, 0.7)
	if !ok {
		return CategoryDecision{}, false
	}
	if rule.Destination == baseRule.Destination && rule.Name == baseRule.Name {
		return CategoryDecision{}, false
	}
	if rule.Name == OtherCategory {
		rule.Name = baseRule.Name
	}

	return CategoryDecision{
		Rule:       rule,
		Source:     "heuristic-extension",
		Reason:     fmt.Sprintf("matched existing .%s files in %s", ext, rule.Destination),
		Confidence: confidence,
	}, true
}

func (c *Categorizer) classifyByContent(name string, current CategoryDecision) (CategoryDecision, bool, error) {
	path := c.resolveExistingPath(name)
	if path == "" {
		return CategoryDecision{}, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return CategoryDecision{}, false, fmt.Errorf("cannot inspect %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return CategoryDecision{}, false, nil
	}

	if !isHintableText(name, info.Size()) {
		return CategoryDecision{}, false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return CategoryDecision{}, false, fmt.Errorf("cannot inspect %q: %w", path, err)
	}
	if len(data) > maxContentHintRead {
		data = data[:maxContentHintRead]
	}

	text := string(data)
	if looksLikeDelimitedTable(text) {
		return CategoryDecision{
			Rule:       CategoryRule{Name: "spreadsheets", Destination: "spreadsheets", Origin: "content"},
			Source:     "content-hint",
			Reason:     "content looks like a delimited table",
			Confidence: 0.93,
		}, true, nil
	}
	if looksLikeCode(text) {
		return CategoryDecision{
			Rule:       CategoryRule{Name: "code", Destination: "code", Origin: "content"},
			Source:     "content-hint",
			Reason:     "content looks like source or structured config text",
			Confidence: 0.88,
		}, true, nil
	}

	if hint := spreadsheetKeywordHint(name, text); hint {
		return CategoryDecision{
			Rule:       CategoryRule{Name: "spreadsheets", Destination: "spreadsheets", Origin: "content"},
			Source:     "content-hint",
			Reason:     "filename and text match spreadsheet-style finance keywords",
			Confidence: 0.79,
		}, true, nil
	}
	if current.Rule.Name == OtherCategory || isAmbiguousText(name) {
		if looksLikeDocument(text) {
			return CategoryDecision{
				Rule:       CategoryRule{Name: "documents", Destination: "documents", Origin: "content"},
				Source:     "content-hint",
				Reason:     "content looks like prose or document text",
				Confidence: 0.7,
			}, true, nil
		}
	}

	return CategoryDecision{}, false, nil
}

func (c *Categorizer) resolveExistingPath(name string) string {
	if filepath.IsAbs(name) {
		if exists, _ := fileExists(name); exists {
			return name
		}
		return ""
	}

	candidates := []string{
		filepath.Join(c.Root, name),
		name,
	}
	for _, candidate := range candidates {
		if exists, _ := fileExists(candidate); exists {
			return candidate
		}
	}
	return ""
}

func (c *Categorizer) directoryProfile() (*DirectoryProfile, error) {
	if c.profile != nil {
		return c.profile, nil
	}

	profile, err := buildDirectoryProfile(c.Root, c.Resolver)
	if err != nil {
		return nil, err
	}
	c.profile = profile
	return c.profile, nil
}

func loadLearningStore(root string) (*LearningStore, error) {
	path := filepath.Join(root, learningManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LearningStore{
				Path: path,
				Data: learningManifest{
					Version:    learningManifestVersion,
					Extensions: make(map[string][]learningPreference),
					Tokens:     make(map[string][]learningPreference),
				},
			}, nil
		}
		return nil, fmt.Errorf("cannot read learning data %q: %w", path, err)
	}

	var manifest learningManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("cannot parse learning data %q: %w", path, err)
	}
	if manifest.Version != 0 && manifest.Version != learningManifestVersion {
		return nil, fmt.Errorf("cannot use learning data %q: unsupported version %d", path, manifest.Version)
	}
	if manifest.Extensions == nil {
		manifest.Extensions = make(map[string][]learningPreference)
	}
	if manifest.Tokens == nil {
		manifest.Tokens = make(map[string][]learningPreference)
	}

	return &LearningStore{Path: path, Data: manifest, Exists: true}, nil
}

func (s *LearningStore) Observe(name string, rule CategoryRule) {
	ext := normalizeExtension(filepath.Ext(name))
	if ext != "" {
		s.Data.Extensions[ext] = incrementPreference(s.Data.Extensions[ext], rule)
	}

	for _, token := range tokenizeName(name) {
		s.Data.Tokens[token] = incrementPreference(s.Data.Tokens[token], rule)
	}

	s.updated = true
}

func (s *LearningStore) Save() error {
	if !s.updated {
		return nil
	}

	s.Data.Version = learningManifestVersion
	s.Data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode learning data: %w", err)
	}
	data = append(data, '\n')

	tempPath := s.Path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("cannot write learning data: %w", err)
	}
	defer os.Remove(tempPath)

	if err := os.Rename(tempPath, s.Path); err != nil {
		return fmt.Errorf("cannot replace learning data: %w", err)
	}
	s.Exists = true
	s.updated = false
	return nil
}

type extensionPreference struct {
	Extension   string
	Category    string
	Destination string
	Count       int
}

func (s *LearningStore) StrongExtensionPreferences() []extensionPreference {
	preferences := make([]extensionPreference, 0)
	for ext := range s.Data.Extensions {
		best, _, ok := s.BestExtension(ext)
		if !ok {
			continue
		}
		preferences = append(preferences, extensionPreference{
			Extension:   ext,
			Category:    best.Category,
			Destination: best.Destination,
			Count:       best.Count,
		})
	}
	return preferences
}

func (s *LearningStore) BestExtension(ext string) (learningPreference, float64, bool) {
	preference, confidence, ok := bestPreference(s.Data.Extensions[ext], 2, 0.6)
	if !ok {
		return learningPreference{}, 0, false
	}
	return preference, 0.85 + confidence*0.1, true
}

func (s *LearningStore) BestTokens(name string) (learningPreference, string, float64, bool) {
	scores := make(map[string]int)
	tokenSource := ""
	for _, token := range tokenizeName(name) {
		prefs := s.Data.Tokens[token]
		if len(prefs) == 0 {
			continue
		}
		if tokenSource == "" {
			tokenSource = token
		}
		for _, pref := range prefs {
			scores[preferenceKey(pref.Category, pref.Destination)] += pref.Count
		}
	}

	key, count, total, ok := strongestKey(scores, 3, 0.65)
	if !ok {
		return learningPreference{}, "", 0, false
	}

	category, destination := splitPreferenceKey(key)
	return learningPreference{Category: category, Destination: destination, Count: count}, tokenSource, 0.8 + share(count, total)*0.1, true
}

func buildDirectoryProfile(root string, resolver CategoryResolver) (*DirectoryProfile, error) {
	profile := &DirectoryProfile{
		ByExtension: make(map[string]map[string]int),
		ByStem:      make(map[string]map[string]int),
		ByToken:     make(map[string]map[string]int),
	}

	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if entry.IsDir() {
			if isHiddenPath(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isHiddenPath(entry.Name()) || !entry.Type().IsRegular() {
			return nil
		}

		destination := filepath.ToSlash(filepath.Dir(rel))
		if destination == "." {
			return nil
		}

		rule := resolver.Resolve(entry.Name())
		if rule.Name == OtherCategory {
			if inferred, ok := inferCategoryFromPath(destination); ok {
				rule.Name = inferred
			}
		}
		rule.Destination = destination
		encoded := encodeRule(rule)

		ext := normalizeExtension(filepath.Ext(entry.Name()))
		if ext != "" {
			incrementCount(profile.ByExtension, ext, encoded)
		}

		stem := normalizedStem(entry.Name())
		if stem != "" {
			incrementCount(profile.ByStem, stem, encoded)
		}
		for _, token := range tokenizeName(entry.Name()) {
			incrementCount(profile.ByToken, token, encoded)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("cannot build directory profile for %q: %w", root, err)
	}

	return profile, nil
}

func inferCategoryFromPath(path string) (string, bool) {
	keywordMap := map[string]string{
		"audio":         "audio",
		"book":          "documents",
		"books":         "documents",
		"budget":        "spreadsheets",
		"budgets":       "spreadsheets",
		"code":          "code",
		"data":          "spreadsheets",
		"deck":          "presentations",
		"design":        "images",
		"docs":          "documents",
		"document":      "documents",
		"documents":     "documents",
		"finance":       "spreadsheets",
		"image":         "images",
		"images":        "images",
		"invoice":       "spreadsheets",
		"music":         "audio",
		"photo":         "images",
		"photography":   "images",
		"photos":        "images",
		"presentation":  "presentations",
		"presentations": "presentations",
		"receipt":       "spreadsheets",
		"recording":     "videos",
		"recordings":    "videos",
		"slide":         "presentations",
		"slides":        "presentations",
		"spreadsheet":   "spreadsheets",
		"spreadsheets":  "spreadsheets",
		"video":         "videos",
		"videos":        "videos",
	}

	for _, token := range tokenizePath(path) {
		if category, ok := keywordMap[token]; ok {
			return category, true
		}
	}
	return "", false
}

func incrementPreference(preferences []learningPreference, rule CategoryRule) []learningPreference {
	for i := range preferences {
		if preferences[i].Category == rule.Name && preferences[i].Destination == rule.Destination {
			preferences[i].Count++
			return preferences
		}
	}
	preferences = append(preferences, learningPreference{
		Category:    rule.Name,
		Destination: rule.Destination,
		Count:       1,
	})
	return preferences
}

func incrementCount(bucket map[string]map[string]int, key, encoded string) {
	if bucket[key] == nil {
		bucket[key] = make(map[string]int)
	}
	bucket[key][encoded]++
}

func bestPreference(preferences []learningPreference, minCount int, minShare float64) (learningPreference, float64, bool) {
	if len(preferences) == 0 {
		return learningPreference{}, 0, false
	}

	sort.Slice(preferences, func(i, j int) bool {
		if preferences[i].Count != preferences[j].Count {
			return preferences[i].Count > preferences[j].Count
		}
		if preferences[i].Category != preferences[j].Category {
			return preferences[i].Category < preferences[j].Category
		}
		return preferences[i].Destination < preferences[j].Destination
	})

	total := 0
	for _, preference := range preferences {
		total += preference.Count
	}

	best := preferences[0]
	if best.Count < minCount || share(best.Count, total) < minShare {
		return learningPreference{}, 0, false
	}

	return best, share(best.Count, total), true
}

func topRule(scores map[string]int, minCount int, minShare float64) (CategoryRule, float64, bool) {
	key, count, total, ok := strongestKey(scores, minCount, minShare)
	if !ok {
		return CategoryRule{}, 0, false
	}

	rule := decodeRule(key)
	return rule, 0.7 + share(count, total)*0.15, true
}

func strongestKey(scores map[string]int, minCount int, minShare float64) (string, int, int, bool) {
	if len(scores) == 0 {
		return "", 0, 0, false
	}

	type scored struct {
		Key   string
		Count int
	}

	items := make([]scored, 0, len(scores))
	total := 0
	for key, count := range scores {
		items = append(items, scored{Key: key, Count: count})
		total += count
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Key < items[j].Key
	})

	if items[0].Count < minCount || share(items[0].Count, total) < minShare {
		return "", 0, 0, false
	}
	return items[0].Key, items[0].Count, total, true
}

func encodeRule(rule CategoryRule) string {
	return preferenceKey(rule.Name, rule.Destination)
}

func decodeRule(encoded string) CategoryRule {
	category, destination := splitPreferenceKey(encoded)
	return CategoryRule{Name: category, Destination: destination, Origin: "adaptive"}
}

func preferenceKey(category, destination string) string {
	return category + "\n" + destination
}

func splitPreferenceKey(key string) (string, string) {
	parts := strings.SplitN(key, "\n", 2)
	if len(parts) == 1 {
		return parts[0], parts[0]
	}
	return parts[0], parts[1]
}

func share(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total)
}

func tokenizeName(name string) []string {
	stem := normalizedStem(name)
	if stem == "" {
		return nil
	}

	parts := splitTokens(stem)
	return filterTokens(parts)
}

func tokenizePath(path string) []string {
	return filterTokens(splitTokens(strings.ToLower(filepath.ToSlash(path))))
}

func normalizedStem(name string) string {
	base := strings.ToLower(filepath.Base(name))
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func splitTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
}

func filterTokens(tokens []string) []string {
	stopwords := map[string]struct{}{
		"copy": {}, "draft": {}, "final": {}, "new": {}, "old": {}, "the": {},
	}
	out := make([]string, 0, len(tokens))
	seen := make(map[string]struct{})
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if len(token) < 3 {
			continue
		}
		if _, ok := stopwords[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func looksLikeDelimitedTable(text string) bool {
	lines := firstNonEmptyLines(text, 6)
	if len(lines) < 2 {
		return false
	}

	for _, delimiter := range []string{",", "\t", ";"} {
		matches := 0
		expectedColumns := 0
		for _, line := range lines {
			columns := strings.Count(line, delimiter) + 1
			if columns < 3 {
				continue
			}
			if expectedColumns == 0 {
				expectedColumns = columns
			}
			if columns == expectedColumns {
				matches++
			}
		}
		if matches >= 2 {
			return true
		}
	}

	return false
}

func looksLikeCode(text string) bool {
	lowered := strings.ToLower(text)
	markers := []string{
		"package ", "func ", "import ", "class ", "def ", "#!/bin/",
		"{", "}", "</", "version:", "apiVersion:", "resource \"",
	}

	hits := 0
	for _, marker := range markers {
		if strings.Contains(lowered, strings.ToLower(marker)) {
			hits++
		}
	}
	return hits >= 2
}

func looksLikeDocument(text string) bool {
	lines := firstNonEmptyLines(text, 8)
	if len(lines) == 0 {
		return false
	}

	proseLines := 0
	for _, line := range lines {
		if len(strings.Fields(line)) >= 5 {
			proseLines++
		}
	}
	return proseLines >= 2
}

func spreadsheetKeywordHint(name, text string) bool {
	keywords := []string{
		"amount", "balance", "budget", "expense", "expenses",
		"forecast", "invoice", "ledger", "price", "qty", "revenue", "subtotal", "total",
	}

	lowered := strings.ToLower(name + "\n" + text)
	matches := 0
	for _, keyword := range keywords {
		if strings.Contains(lowered, keyword) {
			matches++
		}
	}
	return matches >= 2
}

func firstNonEmptyLines(text string, limit int) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, limit)
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == limit {
			break
		}
	}
	return lines
}

func isHintableText(name string, size int64) bool {
	if size > maxContentHintRead*8 {
		return false
	}

	ext := normalizeExtension(filepath.Ext(name))
	switch ext {
	case "", "csv", "log", "md", "text", "tsv", "txt":
		return true
	default:
		return false
	}
}

func isAmbiguousText(name string) bool {
	ext := normalizeExtension(filepath.Ext(name))
	switch ext {
	case "", "log", "text", "txt":
		return true
	default:
		return false
	}
}

func isHiddenPath(name string) bool {
	return strings.HasPrefix(name, ".")
}

func baseConfidenceForRule(name string, rule CategoryRule) float64 {
	if rule.Origin == "config" {
		return 1
	}
	if rule.Name == OtherCategory || isAmbiguousText(name) {
		return 0.4
	}
	return 0.6
}
