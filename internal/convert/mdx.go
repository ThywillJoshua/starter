package convert

import (
    "context"
    "encoding/json"
    "fmt"
    "regexp"
    "os"
    "path/filepath"
    "strings"
)

type ImageRef struct {
    Name string
    Page int
}

func writeMDX(
    ctx context.Context,
    outDir string,
    s Section,
    pageTexts []PageText,
    images []ImageRef,
    siteAllow []string,
    enhancer interface{
        SuggestComponents(ctx context.Context, text string, allow []string, mode string) (string, error)
        Summarize(ctx context.Context, text string, maxTokens int) (string, error)
        Caption(ctx context.Context, imagePath string) (string, error)
    },
) (string, error) {
    title := s.Title
    front := fmt.Sprintf("---\ntitle: \"%s\"\n---\n\n", escapeQuotes(title))
    var b strings.Builder
    b.WriteString(front)
    b.WriteString("# ")
    b.WriteString(title)
    b.WriteString("\n\n")

    // Build content by page, then append images on the same page.
    for _, pt := range pageTexts {
        if t := strings.TrimSpace(pt.Text); t != "" {
            t = stripMarkdownCodeFences(t)
            b.WriteString(t)
            b.WriteString("\n\n")
        }
        // Add images for this page
        for _, img := range images {
            if img.Page != pt.Page {
                continue
            }
            rel := filepath.ToSlash(filepath.Join("images", img.Name))
            alt := "Image"
            if enhancer != nil {
                // Try multimodal caption first
                full := filepath.Join(outDir, "images", img.Name)
                if cap, err := enhancer.Caption(ctx, full); err == nil && strings.TrimSpace(cap) != "" {
                    alt = strings.TrimSpace(cap)
                } else {
                    // Fallback to text-only summarization of local context
                    snippet := title
                    if len(pt.Text) > 0 {
                        if len(pt.Text) > 280 {
                            snippet = pt.Text[:280]
                        } else {
                            snippet = pt.Text
                        }
                    }
                    if sum, err := enhancer.Summarize(ctx, "Generate a short descriptive alt text for an image in a section titled '"+title+"' with this context: \n"+snippet, 30); err == nil && strings.TrimSpace(sum) != "" {
                        alt = strings.TrimSpace(sum)
                    }
                }
            }
            b.WriteString(fmt.Sprintf("![%s](./%s)\n\n", escapeQuotes(alt), rel))
        }
    }

    content := b.String()
    if enhancer != nil {
        if updated, err := enhancer.SuggestComponents(ctx, content, siteAllow, "conservative"); err == nil && updated != "" {
            content = stripMarkdownCodeFences(updated)
        }
    }
    file := filepath.Join(outDir, s.Slug+".mdx")
    if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
        return "", err
    }
    return file, nil
}

func escapeQuotes(s string) string { return strings.ReplaceAll(s, "\"", "\\\"") }

func stripMarkdownCodeFences(s string) string {
    s = strings.TrimSpace(s)

    // Check for opening code fence (```markdown, ```mdx, or just ```)
    if strings.HasPrefix(s, "```") {
        // Find first newline after opening fence
        firstNewline := strings.Index(s, "\n")
        if firstNewline != -1 {
            s = s[firstNewline+1:]
        }
    }

    // Check for closing code fence
    if strings.HasSuffix(s, "```") {
        s = strings.TrimSuffix(s, "```")
        s = strings.TrimSpace(s)
    }

    return s
}

type docsJSON struct {
    Schema     string                 `json:"$schema"`
    Theme      string                 `json:"theme"`
    Name       string                 `json:"name"`
    Colors     map[string]string      `json:"colors,omitempty"`
    Favicon    string                 `json:"favicon,omitempty"`
    Navigation map[string]interface{} `json:"navigation"`
    Logo       map[string]string      `json:"logo,omitempty"`
    Navbar     map[string]interface{} `json:"navbar,omitempty"`
    Contextual map[string]interface{} `json:"contextual,omitempty"`
    Footer     map[string]interface{} `json:"footer,omitempty"`
}

func initializeDocsJSON(path string, siteName string) error {
    if siteName == "" {
        siteName = "Documentation"
    }

    cfg := docsJSON{
        Schema: "https://mintlify.com/docs.json",
        Theme:  "mint",
        Name:   siteName,
        Colors: map[string]string{
            "primary": "#16A34A",
            "light":   "#07C983",
            "dark":    "#15803D",
        },
        Navigation: map[string]interface{}{
            "tabs": []map[string]interface{}{},
        },
    }

    out, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, out, 0o644)
}

func updateDocsJSON(path string, siteName string, sections []Section) error {
    b, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    var cfg docsJSON
    if err := json.Unmarshal(b, &cfg); err != nil {
        return err
    }
    if siteName != "" {
        cfg.Name = siteName
    }

    // Build simple page list from sections (all depth-1, no nesting)
    pages := []interface{}{"index"} // Always include index first
    for _, s := range sections {
        pages = append(pages, s.Slug)
    }

    // Create single Documentation tab with single Manual group
    tabs := []map[string]interface{}{
        {
            "tab": "Documentation",
            "groups": []map[string]interface{}{
                {
                    "group": "Manual",
                    "pages": pages,
                },
            },
        },
    }

    cfg.Navigation = map[string]interface{}{
        "tabs": tabs,
    }

    out, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, out, 0o644)
}

// buildPagesRecursive converts a Section tree into a Mintlify-compatible pages array.
// Each entry is either a string slug (leaf page) or a nested group object.
func buildPagesRecursive(s Section) []interface{} {
    var pages []interface{}
    // include the section's own page first
    pages = append(pages, s.Slug)
    for _, c := range s.Children {
        if len(c.Children) == 0 {
            pages = append(pages, c.Slug)
            continue
        }
        pages = append(pages, map[string]interface{}{
            "group": groupLabel(c),
            "pages": buildPagesRecursive(c),
        })
    }
    return pages
}

func groupLabel(s Section) string {
    // If top-level alpha-only numbering, prefix with "Appendix" for clarity.
    alphaOnly := regexp.MustCompile(`^[A-Z]+$`)
    if s.Depth == 1 && alphaOnly.MatchString(s.Number) {
        return "Appendix " + s.Number + " " + s.Title
    }
    return s.Number + " " + s.Title
}
