package convert

import (
    "regexp"
    "sort"
    "strconv"
    "strings"
)

// Patterns for ToC entries: numeric, roman numerals, alphabetic appendices, and explicit Appendix prefix.
var (
    tocNumRe      = regexp.MustCompile(`^\s*(\d+(?:\.\d+)*)\s+(.+?)\s+(\d+)\s*$`)
    tocRomanRe    = regexp.MustCompile(`^\s*([IVXLCDM]+)(?:\.([0-9]+))?\s+(.+?)\s+(\d+)\s*$`)
    tocAlphaRe    = regexp.MustCompile(`^\s*([A-Z](?:\.[0-9]+)*)\s+(.+?)\s+(\d+)\s*$`)
    tocAppendixRe = regexp.MustCompile(`^\s*(?:Appendix|APPENDIX)\s+([A-Z](?:\.[0-9]+)*)\s+(.+?)\s+(\d+)\s*$`)
)

type tocEntry struct {
    Number string // display number token (e.g., 1.2, I, A.1)
    Title  string
    Page   int
    Depth  int
}

func parseToCLines(lines []string) []tocEntry {
    var out []tocEntry
    for _, line := range lines {
        line = normalizeDotLeaders(line)
        if e, ok := matchToC(line); ok {
            out = append(out, e)
        }
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].Page < out[j].Page })
    return out
}

func matchToC(line string) (tocEntry, bool) {
    if m := tocAppendixRe.FindStringSubmatch(line); len(m) == 4 {
        p, _ := strconv.Atoi(m[3])
        key := m[1]
        depth := strings.Count(key, ".") + 1
        return tocEntry{Number: key, Title: strings.TrimSpace(m[2]), Page: p, Depth: depth}, true
    }
    if m := tocNumRe.FindStringSubmatch(line); len(m) == 4 {
        p, _ := strconv.Atoi(m[3])
        depth := strings.Count(m[1], ".") + 1
        return tocEntry{Number: m[1], Title: strings.TrimSpace(m[2]), Page: p, Depth: depth}, true
    }
    if m := tocAlphaRe.FindStringSubmatch(line); len(m) == 4 {
        p, _ := strconv.Atoi(m[3])
        depth := strings.Count(m[1], ".") + 1
        return tocEntry{Number: m[1], Title: strings.TrimSpace(m[2]), Page: p, Depth: depth}, true
    }
    if m := tocRomanRe.FindStringSubmatch(line); len(m) == 5 {
        p, _ := strconv.Atoi(m[4])
        // depth is 1 if only roman; if has .<n>, treat as depth 2
        depth := 1
        num := m[1]
        if m[2] != "" {
            num = num + "." + m[2]
            depth = 2
        }
        return tocEntry{Number: num, Title: strings.TrimSpace(m[3]), Page: p, Depth: depth}, true
    }
    return tocEntry{}, false
}

func isToCLine(s string) bool {
    s = normalizeDotLeaders(s)
    return tocAppendixRe.MatchString(s) || tocNumRe.MatchString(s) || tocAlphaRe.MatchString(s) || tocRomanRe.MatchString(s)
}

func normalizeDotLeaders(s string) string {
    s = strings.ReplaceAll(s, "\u2022", " ")
    s = strings.ReplaceAll(s, "\u00B7", " ")
    s = strings.ReplaceAll(s, "\u2026", " ... ")
    s = strings.ReplaceAll(s, "·", " ")
    s = strings.ReplaceAll(s, "…", " ... ")
    s = strings.ReplaceAll(s, "..........................................................................................................", " ")
    s = strings.ReplaceAll(s, " . . . ", " ")
    s = strings.Join(strings.Fields(s), " ")
    return s
}

func buildSections(entries []tocEntry, maxDepth int) []Section {
    if maxDepth <= 0 {
        maxDepth = 10
    }
    n := len(entries)
    var sections []Section
    for i, e := range entries {
        if e.Depth > maxDepth {
            continue
        }
        end := e.Page
        if i < n-1 {
            end = entries[i+1].Page - 1
            if end < e.Page {
                end = e.Page
            }
        }
        slug := slugify(e.Number + "-" + e.Title)
        sections = append(sections, Section{Number: e.Number, Title: e.Title, Start: e.Page, End: end, Depth: e.Depth, Slug: slug})
    }
    return sections
}
