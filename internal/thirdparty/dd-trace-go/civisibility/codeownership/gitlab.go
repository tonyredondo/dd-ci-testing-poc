// Copyright 2017 Datadog, Inc. Licensed under the Apache License, Version 2.0.
// Go adaptation Copyright 2026 Datadog, Inc.
// Port of CodeOwners.GitLab.cs; see README.md.
package codeownership

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

type gitLabParser struct {
	sections []section
	named    map[string]int
	current  int
	defaults []string
}

func (p *gitLabParser) parseLine(raw string, diagnostics *int) {
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "^[") {
		name, defaults, parsed, bad := parseSectionHeader(raw)
		if bad || !parsed {
			(*diagnostics)++
		}
		if !parsed {
			return
		}
		p.defaults = defaults
		key := foldSectionName(name)
		index, found := p.named[key]
		if !found {
			index = len(p.sections)
			p.named[key] = index
			p.sections = append(p.sections, section{})
		}
		p.current = index
		return
	}
	if raw[0] == '#' {
		return
	}
	pattern, text := splitRule(raw)
	exclusion := strings.HasPrefix(pattern, "!")
	if exclusion {
		pattern = pattern[1:]
	}
	if pattern == "" {
		(*diagnostics)++
		return
	}
	var owners []string
	valid := true
	if !exclusion {
		owners, valid = gitLabOwners(text)
		if text == "" {
			owners = p.defaults
		}
		if len(owners) == 0 {
			valid = false
		}
	}
	r, ok := compileRule(pattern, GitLab, owners, exclusion)
	if !ok {
		(*diagnostics)++
		return
	}
	if !valid {
		(*diagnostics)++
	}
	r.key = gitLabPatternKey(pattern)
	p.sections[p.current].rules = append(p.sections[p.current].rules, r)
}
func (p *gitLabParser) finish() []section {
	for i := range p.sections {
		rules := p.sections[i].rules
		slices.Reverse(rules)
		seen := make(map[string]bool, len(rules))
		out := rules[:0]
		for _, r := range rules {
			if !seen[r.key] {
				seen[r.key] = true
				r.key = ""
				out = append(out, r)
			}
		}
		clear(rules[len(out):])
		p.sections[i].rules = out
		for _, r := range out {
			if r.exclusion {
				p.sections[i].hasExclusions = true
				break
			}
		}
	}
	return p.sections
}

// A recognizable header changes the active section even when its suffix is
// malformed. Only the recognized owner span supplies defaults, as in GitLab.
func parseSectionHeader(raw string) (string, []string, bool, bool) {
	i := 0
	optional := raw[0] == '^'
	if optional {
		i++
	}
	if i >= len(raw) || raw[i] != '[' {
		return "", nil, false, false
	}
	close := strings.IndexByte(raw[i+1:], ']')
	if close < 0 {
		return "", nil, false, false
	}
	close += i + 1
	name := strings.TrimSpace(raw[i+1 : close])
	i = close + 1
	bad := name == ""
	approvalRecognized := false
	if i < len(raw) && raw[i] == '[' {
		end := strings.IndexByte(raw[i+1:], ']')
		if end >= 0 {
			end += i + 1
			number := raw[i+1 : end]
			recognized := true
			for _, r := range number {
				if !unicode.IsSpace(r) && !sectionDigit(r) {
					recognized = false
					break
				}
			}
			if recognized {
				approvalRecognized = true
				n, err := strconv.ParseInt(number, 10, 32)
				if err != nil || optional && n > 0 {
					bad = true
				}
				i = end + 1
			}
		}
	}
	strict := true
	if i < len(raw) {
		r, _ := utf8.DecodeRuneInString(raw[i:])
		strict = unicode.IsSpace(r)
	}
	end := i
	for end < len(raw) {
		r, size := utf8.DecodeRuneInString(raw[end:])
		if !sectionOwnerCharacter(r) {
			break
		}
		end += size
	}
	owners, valid := gitLabOwners(raw[i:end])
	// Approval whitespace is accepted by the permissive parser, but not by the
	// strict header grammar. Empty/malformed approval counts are also diagnosed.
	if approvalRecognized {
		start := strings.Index(raw, "][") + 2
		finish := strings.IndexByte(raw[start:], ']') + start
		if start >= 2 && finish >= start {
			for _, r := range raw[start:finish] {
				if !sectionDigit(r) {
					strict = false
				}
			}
		}
	}
	return name, owners, true, bad || !strict || end < len(raw) || !valid
}
func sectionDigit(r rune) bool { return r <= 0xffff && unitClasses(uint16(r))&2 != 0 }
func sectionOwnerCharacter(r rune) bool {
	return r <= 0xffff && (unitClasses(uint16(r)) != 0 || strings.ContainsRune("@.-/", r))
}

// Owner extraction uses UTF-16 indices, like the upstream char-based tokenizer.
// In particular, supplementary letters are not regex word characters here,
// and the 100/255 email limits count char units rather than Unicode code points.
func gitLabOwners(text string) ([]string, bool) {
	var owners []string
	seen := make(map[string]bool)
	valid := true
	add := func(owner string) {
		if !seen[owner] {
			seen[owner] = true
			owners = append(owners, owner)
		}
	}
	for token := range strings.FieldsFuncSeq(text, ownerSeparator) {
		var storage [128]uint16
		units := utf16Units(token, storage[:])
		start, end, ok := findNamespace(units, 0)
		if ok && start == 0 && end == len(units) || validGitLabRole(units) {
			add(token)
			continue
		}
		found := false
		for search := 0; ; {
			start, end, ok = findNamespace(units, search)
			if !ok {
				break
			}
			add(string(utf16.Decode(units[start:end])))
			found = true
			search = end
		}
		for at := 0; at+1 < len(units); at++ {
			if units[at] != '@' || units[at+1] != '@' || at > 0 && (regexWordCharacter(units[at-1]) || units[at-1] == '@') {
				continue
			}
			end := at + 2
			for end < len(units) && !unicode.IsSpace(rune(units[end])) {
				end++
			}
			if validGitLabRole(units[at:end]) {
				add(string(utf16.Decode(units[at:end])))
				found = true
			}
		}
		for search := 0; ; {
			start, end, ok = findEmail(units, search)
			if !ok {
				break
			}
			email := units[start:end]
			if _, _, hasNamespace := findNamespace(email, 0); !hasNamespace {
				add(string(utf16.Decode(email)))
				found = true
			}
			search = end
		}
		if !found {
			valid = false
		}
	}
	return owners, valid
}
func validGitLabRole(units []uint16) bool {
	if len(units) < 3 || units[0] != '@' || units[1] != '@' {
		return false
	}
	// Roles contain ASCII letters. OrdinalIgnoreCase does not fold dotted I,
	// dotless I, or long s into ASCII as a Unicode lower-case conversion might.
	for _, role := range []string{"developer", "developers", "maintainer", "maintainers", "owner", "owners"} {
		if len(units)-2 != len(role) {
			continue
		}
		equal := true
		for i, unit := range units[2:] {
			if unit >= 'A' && unit <= 'Z' {
				unit += 'a' - 'A'
			}
			if unit != uint16(role[i]) {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}
	return false
}
func findNamespace(token []uint16, start int) (int, int, bool) {
	for at := start; at < len(token); at++ {
		if token[at] != '@' || at+1 == len(token) {
			continue
		}
		if at > 0 && (wordCharacter(token[at-1]) || token[at-1] == '@') {
			continue
		}
		if !namespaceStart(token[at+1]) {
			continue
		}
		segmentStart, lastEnd := at+1, -1
		for i := segmentStart; i < len(token); i++ {
			char := token[i]
			if char == '/' {
				if i == segmentStart || lastEnd != i {
					break
				}
				segmentStart = i + 1
				continue
			}
			if i == segmentStart && !namespaceStart(char) || !namespaceStart(char) && char != '-' {
				break
			}
			if asciiUnit(char) || char == '_' || char == '-' {
				lastEnd = i + 1
			}
		}
		if lastEnd > at+1 {
			return at, lastEnd, true
		}
	}
	return 0, 0, false
}
func asciiUnit(char uint16) bool          { return char < 128 && asciiWord(byte(char)) }
func namespaceStart(char uint16) bool     { return asciiUnit(char) || char == '_' || char == '.' }
func wordCharacter(char uint16) bool      { return unitClasses(char)&3 != 0 || char == '_' }
func regexWordCharacter(char uint16) bool { return unitClasses(char)&(1|2|8) != 0 }
func findEmail(token []uint16, search int) (int, int, bool) {
	for at := search; at < len(token); at++ {
		if token[at] != '@' {
			continue
		}
		local, length := at, 0
		for local > search && length < 100 {
			r := rune(token[local-1])
			if r == '@' || unicode.IsSpace(r) {
				break
			}
			local--
			length++
		}
		if length == 0 {
			continue
		}
		end, lastWord := at+1, -1
		limit := min(len(token), end+255)
		for end < limit {
			if token[end] == '@' || unicode.IsSpace(rune(token[end])) {
				break
			}
			if regexWordCharacter(token[end]) {
				lastWord = end + 1
			}
			end++
		}
		if lastWord > at+1 {
			return local, lastWord, true
		}
	}
	return 0, 0, false
}
func gitLabPatternKey(pattern string) string {
	if pattern == "*" {
		return "/**/*"
	}
	key := pattern
	if strings.ContainsRune(pattern, '\\') {
		var b strings.Builder
		b.Grow(len(pattern))
		for i := 0; i < len(pattern); i++ {
			if pattern[i] == '\\' && i+1 < len(pattern) {
				next, _ := utf8.DecodeRuneInString(pattern[i+1:])
				if i == 0 && next == '#' || unicode.IsSpace(next) {
					i++
				}
			}
			b.WriteByte(pattern[i])
		}
		key = b.String()
	}
	if !strings.HasPrefix(key, "/") {
		key = "/**/" + key
	}
	if strings.HasSuffix(key, "/") {
		key += "**/*"
	}
	return key
}

// .NET OrdinalIgnoreCase uses invariant upper-case mappings. The ASCII range
// intentionally stays distinct from long s and dotless i; Kelvin's upper-case
// mapping is itself. This is narrower than Go's SimpleFold equivalence cycles.
func foldSectionName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '\u017f' || r == '\u0131' {
			return r
		}
		return invariantUpper(r)
	}, name)
}
