package convert

import (
    "regexp"
    "strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9\-]+`)

func slugify(s string) string {
    s = strings.ToLower(s)
    s = strings.TrimSpace(s)
    s = strings.ReplaceAll(s, " ", "-")
    s = strings.ReplaceAll(s, "/", "-")
    s = strings.ReplaceAll(s, ".", "-")
    s = nonSlug.ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    for strings.Contains(s, "--") {
        s = strings.ReplaceAll(s, "--", "-")
    }
    return s
}

// transformTables attempts to identify simple space-aligned tables and convert them to Markdown tables.
func transformTables(text string) string {
    lines := strings.Split(text, "\n")
    var out []string
    i := 0
    for i < len(lines) {
        // Find a block of lines that look like columns (split by 2+ spaces)
        start := i
        colsCount := 0
        var block [][]string
        for i < len(lines) {
            ln := strings.TrimRight(lines[i], " ")
            if ln == "" {
                break
            }
            parts := splitBy2Spaces(ln)
            if len(parts) < 2 {
                break
            }
            if colsCount == 0 {
                colsCount = len(parts)
            }
            if len(parts) != colsCount {
                break
            }
            block = append(block, parts)
            // stop growing overly large tables (safety)
            if len(block) > 50 {
                break
            }
            i++
        }
        if len(block) >= 2 && colsCount >= 2 {
            // Render as Markdown table
            header := block[0]
            out = append(out, "| "+strings.Join(trimAll(header), " | ")+" |")
            var sep []string
            for range header {
                sep = append(sep, "---")
            }
            out = append(out, "| "+strings.Join(sep, " | ")+" |")
            for _, row := range block[1:] {
                out = append(out, "| "+strings.Join(trimAll(row), " | ")+" |")
            }
            // Skip the newline at i (if present)
            if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
                out = append(out, "")
                i++
            }
            continue
        }
        // No table block; add original line and advance
        for j := start; j <= i && j < len(lines); j++ {
            out = append(out, lines[j])
        }
        i++
    }
    return strings.Join(out, "\n")
}

var twoPlusSpaces = regexp.MustCompile(`\s{2,}`)

func splitBy2Spaces(s string) []string {
    return twoPlusSpaces.Split(strings.TrimSpace(s), -1)
}

func trimAll(a []string) []string {
    out := make([]string, len(a))
    for i, v := range a {
        out[i] = strings.TrimSpace(v)
    }
    return out
}
