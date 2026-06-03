package modelreg

import (
	"regexp"
	"strings"
)

// Candidate-name normalization and matching. A model id is expanded into ordered
// candidate names (most specific first) so it can match registry entries that use
// different separators, quantization suffixes, or provider prefixes. Go's regexp
// has no lookahead, so the letter/digit separator transforms use explicit rune scans.

var (
	quantPattern     = regexp.MustCompile(`(?i)[-_](?:q\d[_a-z0-9]*|fp16|fp32|bf16|f16|f32|iq\d[_a-z0-9]*)$`)
	paramSizePattern = regexp.MustCompile(`(?i)^(\d+\.?\d*[bm](?:-a\d+[bm])?)`)
)

var providerAliases = map[string][]string{
	"claude":     {"anthropic"},
	"gemini":     {"google"},
	"openai":     {"openai"},
	"openrouter": {"openrouter"},
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isDigit(b byte) bool  { return b >= '0' && b <= '9' }

// insertDashLetterDigit inserts '-' between a letter and an immediately following
// digit (TS: /([a-z])(?=\d)/ → "$1-").
func insertDashLetterDigit(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteByte(s[i])
		if isLetter(s[i]) && i+1 < len(s) && isDigit(s[i+1]) {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// removeDashLetterDigit drops a '-' sitting between a letter and a digit
// (TS: /([a-z])-(?=\d)/ → "$1").
func removeDashLetterDigit(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '-' && i > 0 && isLetter(s[i-1]) && i+1 < len(s) && isDigit(s[i+1]) {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// digitDashDigitToDot turns "digit-digit" into "digit.digit"
// (TS: /(\d)-(?=\d)/ → "$1.").
func digitDashDigitToDot(s string) string {
	bs := []byte(s)
	for i := 0; i+1 < len(bs); i++ {
		if bs[i] == '-' && i > 0 && isDigit(bs[i-1]) && isDigit(bs[i+1]) {
			bs[i] = '.'
		}
	}
	return string(bs)
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func buildBaseVariants(base string) []string {
	initial := unique([]string{base, insertDashLetterDigit(base), removeDashLetterDigit(base)})
	var expanded []string
	for _, v := range initial {
		expanded = append(expanded, v, strings.ReplaceAll(v, ".", "-"), digitDashDigitToDot(v))
	}
	return unique(expanded)
}

// buildCandidates produces candidate names ordered most-specific first.
func buildCandidates(modelID string) []string {
	full := strings.ToLower(modelID)
	afterSlash := modelID
	if i := strings.LastIndexByte(modelID, '/'); i >= 0 {
		afterSlash = modelID[i+1:]
	}
	lower := strings.ToLower(afterSlash)

	if !strings.Contains(lower, ":") {
		variants := buildBaseVariants(lower)
		if full == lower {
			return unique(variants)
		}
		return unique(append([]string{full}, variants...))
	}

	base, tag, _ := strings.Cut(lower, ":")
	tagBaseVariants := buildBaseVariants(base)
	if tag == "" || tag == "latest" {
		return tagBaseVariants
	}

	cleanTag := strings.TrimSuffix(quantPattern.ReplaceAllString(tag, ""), "-")
	var candidates []string
	if m := paramSizePattern.FindStringSubmatch(cleanTag); m != nil {
		paramSize := m[1]
		rest := strings.TrimPrefix(cleanTag[len(paramSize):], "-")
		for _, bv := range tagBaseVariants {
			if rest != "" {
				candidates = append(candidates, bv+":"+paramSize+"-"+rest, bv+"-"+paramSize+"-"+rest)
			} else {
				candidates = append(candidates, bv+":"+paramSize)
			}
			candidates = append(candidates, bv+"-"+paramSize)
		}
	} else if cleanTag != "" {
		for _, bv := range tagBaseVariants {
			candidates = append(candidates, bv+":"+cleanTag, bv+"-"+cleanTag)
		}
	}
	candidates = append(candidates, tagBaseVariants...)
	return unique(candidates)
}

func buildProviderCandidates(providerID string, modelCandidates []string) []string {
	if providerID == "" {
		return nil
	}
	aliases := append([]string{providerID}, providerAliases[providerID]...)
	var out []string
	for _, p := range aliases {
		for _, c := range modelCandidates {
			out = append(out, strings.ToLower(p+"/"+c))
		}
	}
	return out
}

func extractBaseName(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		id = id[i+1:]
	}
	return strings.ToLower(id)
}

// isMatch reports a prefix match on a boundary (-, ., :, or end).
func isMatch(registryBaseName, candidate string) bool {
	if registryBaseName == candidate {
		return true
	}
	if strings.HasPrefix(registryBaseName, candidate) {
		if len(registryBaseName) == len(candidate) {
			return true
		}
		next := registryBaseName[len(candidate)]
		return next == '-' || next == '.' || next == ':'
	}
	return false
}

// mergeLimits fills missing fields from later limits (first non-zero wins per field).
func mergeLimits(limits ...Limits) (Limits, bool) {
	var merged Limits
	for _, l := range limits {
		if merged.ContextWindow == 0 {
			merged.ContextWindow = l.ContextWindow
		}
		if merged.MaxOutputTokens == 0 {
			merged.MaxOutputTokens = l.MaxOutputTokens
		}
	}
	return merged, merged.ContextWindow != 0 || merged.MaxOutputTokens != 0
}
