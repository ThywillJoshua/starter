package convert

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

type runResult struct {
    Sections []Section `json:"sections"`
    Images   int       `json:"images_extracted"`
    OutDir   string    `json:"out_dir"`
}

func Run(ctx context.Context, pdfPath string, cfg Config) (runResult, error) {
    if cfg.OutDir == "" {
        cfg.OutDir = "."
    }
    if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
        return runResult{}, err
    }

    // Initialize minimal docs.json if it doesn't exist
    docsPath := filepath.Join(cfg.OutDir, "docs.json")
    if _, err := os.Stat(docsPath); os.IsNotExist(err) {
        if err := initializeDocsJSON(docsPath, cfg.SiteName); err != nil {
            return runResult{}, fmt.Errorf("failed to initialize docs.json: %w", err)
        }
    }

    // Gemini-exclusive path: use AI to extract structure and content
    if cfg.AIExclusive && cfg.Enhancer != nil {
        fmt.Println("🤖 Using Gemini AI to extract PDF structure...")
        doc, err := cfg.Enhancer.ExtractStructure(ctx, pdfPath, cfg.MaxDepth, cfg.ToCPages)
        if err != nil {
            return runResult{}, fmt.Errorf("AI extraction failed: %w", err)
        }
        if len(doc.Sections) == 0 {
            return runResult{}, fmt.Errorf("AI extraction returned no sections - check if PDF is readable or try without --ai-exclusive")
        }

        fmt.Printf("✅ Extracted %d sections from PDF using Gemini\n", len(doc.Sections))

        // Write MDX files from structured doc - ONLY for depth-1 sections
        imgDir := filepath.Join(cfg.OutDir, "images")
        imgCount := 0
        if cfg.KeepImages {
            if n, _ := extractImages(pdfPath, imgDir); n > 0 { imgCount = n }
        }

        // Process only depth-1 sections (top-level)
        // Each depth-1 section's text already contains all subsections formatted as markdown headers
        var sections []Section
        for _, s := range doc.Sections {
            // Only process depth-1 sections
            if s.Depth != 1 {
                continue
            }

            sec := Section{
                Number: s.Number,
                Title:  s.Title,
                Start:  s.Start,
                End:    s.End,
                Depth:  s.Depth,
                Slug:   slugify(s.Title), // Use title only, not number
            }
            sections = append(sections, sec)

            // Create single MDX file with all content (subsections already in s.Text as markdown headers)
            pageTexts := []PageText{{Page: s.Start, Text: s.Text}}
            var imgs []ImageRef
            if cfg.KeepImages {
                imgs = discoverImagesForRange(imgDir, s.Start, s.End)
            }
            _, _ = writeMDX(ctx, cfg.OutDir, sec, pageTexts, imgs, []string{"callout","steps","accordion"}, cfg.Enhancer)
        }

        if err := updateDocsJSON(docsPath, cfg.SiteName, sections); err != nil { return runResult{}, err }
        if err := writeIndex(cfg.OutDir, sections); err != nil { return runResult{}, err }
        return runResult{Sections: sections, Images: imgCount, OutDir: cfg.OutDir}, nil
    }

    pages, err := extractTextPerPage(pdfPath)
    if err != nil {
        return runResult{}, err
    }

    var tocLines []string
    if cfg.UseToC {
        tocLines = findToCMultiPage(pages, cfg.ToCPages)
        if cfg.Enhancer != nil && len(tocLines) > 0 {
            if repaired, err := cfg.Enhancer.RepairToC(ctx, tocLines); err == nil && len(repaired) > 0 {
                tocLines = repaired
            }
        }
    }
    var sections []Section
    if len(tocLines) > 0 {
        entries := parseToCLines(tocLines)
        sections = buildSections(entries, cfg.MaxDepth)
    } else {
        sections = fallbackSections(pages, cfg.FallbackSplit)
    }

    imgDir := filepath.Join(cfg.OutDir, "images")
    imgCount := 0
    if cfg.KeepImages {
        if n, _ := extractImages(pdfPath, imgDir); n > 0 {
            imgCount = n
        }
    }

    for i, s := range sections {
        pageTexts := collectTextByPage(pages, s.Start, s.End)
        if cfg.SlugPrefix != "" {
            s.Slug = slugify(cfg.SlugPrefix + "-" + s.Slug)
        }
        sections[i] = s
        var imgs []ImageRef
        if cfg.KeepImages {
            imgs = discoverImagesForRange(imgDir, s.Start, s.End)
        }
        if _, err := writeMDX(ctx, cfg.OutDir, s, pageTexts, imgs, []string{"callout", "steps", "accordion"}, cfg.Enhancer); err != nil {
            return runResult{}, err
        }
    }

    // Build hierarchy for nested navigation
    tree := buildHierarchy(sections)
    if err := updateDocsJSON(docsPath, cfg.SiteName, tree); err != nil {
        return runResult{}, err
    }
    if err := writeIndex(cfg.OutDir, filterTopLevel(sections)); err != nil {
        return runResult{}, err
    }
    return runResult{Sections: sections, Images: imgCount, OutDir: cfg.OutDir}, nil
}

type PageText struct {
    Page int
    Text string
}

func collectTextByPage(pages []string, start, end int) []PageText {
    if start < 1 {
        start = 1
    }
    if end > len(pages) {
        end = len(pages)
    }
    var out []PageText
    for i := start; i <= end; i++ {
        t := strings.TrimSpace(pages[i-1])
        t = transformTables(t)
        out = append(out, PageText{Page: i, Text: t})
    }
    return out
}

func findToCBlock(pages []string) []string {
    headerRe := regexp.MustCompile(`(?i)\btable of contents\b|^\s*contents\s*$`)
    pageNumRe := regexp.MustCompile(`\b\d+\s*$`)
    var lines []string
    for i := 0; i < len(pages) && i < 6; i++ {
        p := pages[i]
        if headerRe.MatchString(p) || strings.Contains(strings.ToLower(p), "table of contents") {
            chunks := strings.Split(p, "\n")
            for _, ln := range chunks {
                if pageNumRe.MatchString(strings.TrimSpace(ln)) {
                    lines = append(lines, ln)
                }
            }
        }
    }
    if len(lines) == 0 {
        // Fallback: scan early pages for lines with trailing page numbers and leading numbering
        for i := 0; i < len(pages) && i < 6; i++ {
            for _, ln := range strings.Split(pages[i], "\n") {
                ln = strings.TrimSpace(ln)
                if isToCLine(ln) {
                    lines = append(lines, ln)
                }
            }
        }
    }
    return lines
}

// findToCMultiPage scans the first N pages to collect ToC lines across consecutive pages.
func findToCMultiPage(pages []string, n int) []string {
    if n <= 0 {
        n = 8
    }
    // First try the simple detector.
    lines := findToCBlock(pages)
    if len(lines) > 0 {
        // Try to expand to subsequent pages that continue ToC lines
        lastPage := 0
        // find which early page produced lines
        headerRe := regexp.MustCompile(`(?i)\btable of contents\b|^\s*contents\s*$`)
        for i := 0; i < len(pages) && i < n; i++ {
            p := pages[i]
            if headerRe.MatchString(p) || matchToCLines(p) {
                lastPage = i
                break
            }
        }
        extend := n / 2
        if extend < 2 {
            extend = 2
        }
        for i := lastPage + 1; i < len(pages) && i <= lastPage+extend; i++ {
            added := false
            for _, ln := range strings.Split(pages[i], "\n") {
                ln = strings.TrimSpace(ln)
                if ln == "" {
                    continue
                }
                if isToCLine(ln) {
                    lines = append(lines, ln)
                    added = true
                }
            }
            // stop if a page adds nothing, assuming ToC ended
            if !added {
                break
            }
        }
        return lines
    }
    // If none found, attempt scan across first 8 pages for any matching lines
    var out []string
    for i := 0; i < len(pages) && i < n; i++ {
        for _, ln := range strings.Split(pages[i], "\n") {
            ln = strings.TrimSpace(ln)
            if ln == "" {
                continue
            }
            if isToCLine(ln) {
                out = append(out, ln)
            }
        }
    }
    return out
}

func matchToCLines(p string) bool {
    for _, ln := range strings.Split(p, "\n") {
        if isToCLine(strings.TrimSpace(ln)) {
            return true
        }
    }
    return false
}

func fallbackSections(pages []string, mode string) []Section {
    var out []Section
    if mode == "heading" {
        // Basic heading-based split: start new section when a line looks like a heading
        heading := regexp.MustCompile(`^[A-Z][A-Za-z0-9 ,\-/()]{3,}$`)
        cur := Section{Number: "1", Title: "Section 1", Start: 1, End: 1, Depth: 1, Slug: slugify("1-section-1")}
        idx := 1
        for i := 1; i <= len(pages); i++ {
            lines := strings.Split(pages[i-1], "\n")
            for _, ln := range lines {
                if heading.MatchString(strings.TrimSpace(ln)) && i != cur.Start {
                    cur.End = i - 1
                    out = append(out, cur)
                    idx++
                    cur = Section{Number: fmt.Sprintf("%d", idx), Title: strings.TrimSpace(ln), Start: i, End: i, Depth: 1, Slug: slugify(fmt.Sprintf("%d-%s", idx, ln))}
                    break
                }
            }
            cur.End = i
        }
        out = append(out, cur)
        return out
    }
    for i := 1; i <= len(pages); i++ {
        t := fmt.Sprintf("Page %d", i)
        out = append(out, Section{Number: fmt.Sprintf("%d", i), Title: t, Start: i, End: i, Depth: 1, Slug: slugify(t)})
    }
    return out
}

func filterTopLevel(sections []Section) []Section {
    var out []Section
    for _, s := range sections {
        if s.Depth == 1 {
            out = append(out, s)
        }
    }
    return out
}

func discoverImagesForRange(dir string, start, end int) []ImageRef {
    var imgs []ImageRef
    entries, _ := os.ReadDir(dir)
    for _, e := range entries {
        name := e.Name()
        // pdfcpu names like p<page>_img_<n>.<ext>
        pg := parsePageFromName(name)
        if pg >= start && pg <= end {
            imgs = append(imgs, ImageRef{Name: name, Page: pg})
        }
    }
    return imgs
}

func parsePageFromName(name string) int {
    // Try several common patterns
    re := regexp.MustCompile(`p(\d+)`)
    m := re.FindStringSubmatch(name)
    if len(m) == 2 {
        return atoi(m[1])
    }
    re2 := regexp.MustCompile(`page-(\d+)`)
    m = re2.FindStringSubmatch(name)
    if len(m) == 2 {
        return atoi(m[1])
    }
    return 0
}

func atoi(s string) int {
    n := 0
    for _, r := range s {
        if r < '0' || r > '9' {
            continue
        }
        n = n*10 + int(r-'0')
    }
    return n
}

func writeIndex(outDir string, tops []Section) error {
    var b strings.Builder
    b.WriteString("---\ntitle: \"Introduction\"\ndescription: \"Auto-generated overview\"\n---\n\n")
    b.WriteString("## Sections\n\n")
    for _, s := range tops {
        b.WriteString(fmt.Sprintf("- [%s %s](./%s)\n", s.Number, s.Title, s.Slug))
    }
    path := filepath.Join(outDir, "index.mdx")
    return os.WriteFile(path, []byte(b.String()), 0o644)
}
