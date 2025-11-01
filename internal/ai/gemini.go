package ai

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "mime"
    "os"
    "path/filepath"
    "strings"

    genai "google.golang.org/genai"
)

type Gemini struct {
    client           *genai.Client
    model            string
    ComponentsMode  string
    ComponentsAllow []string
}

func NewGemini(ctx context.Context, apiKey, model string) (*Gemini, error) {
    if apiKey == "" {
        return nil, errors.New("missing GOOGLE_API_KEY")
    }
    if model == "" {
        model = "gemini-2.5-flash"
    }
    c, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
    if err != nil {
        return nil, err
    }
    return &Gemini{client: c, model: model, ComponentsMode: "conservative"}, nil
}

func (g *Gemini) prompt(ctx context.Context, text string) (string, error) {
    res, err := g.client.Models.GenerateContent(ctx, g.model, []*genai.Content{
        genai.NewContentFromText(text, genai.RoleUser),
    }, nil)
    if err != nil {
        return "", err
    }
    return res.Text(), nil
}

func (g *Gemini) RepairToC(ctx context.Context, raw []string) ([]string, error) {
    if g.client == nil {
        return raw, nil
    }
    joined := "Fix and normalize this Table of Contents to one entry per line as 'NUMBER TITLE .... PAGE', keep order, no extra text.\n\n" + joinLines(raw)
    out, err := g.prompt(ctx, joined)
    if err != nil || out == "" {
        return raw, nil
    }
    return splitLines(out), nil
}

func (g *Gemini) Summarize(ctx context.Context, text string, maxTokens int) (string, error) {
    if g.client == nil {
        return "", nil
    }
    prompt := "Summarize in one sentence (max 25 words):\n\n" + text
    return g.prompt(ctx, prompt)
}

func (g *Gemini) SuggestComponents(ctx context.Context, text string, allow []string, mode string) (string, error) {
    if g.client == nil {
        return "", nil
    }
    prompt := "Given this Markdown content, propose minimal Mintlify MDX component annotations strictly from this allowlist: " + joinLines(allow) + ". Return ONLY the updated Markdown content - DO NOT wrap your response in ```markdown or ``` code fences.\n\n" + text
    return g.prompt(ctx, prompt)
}

func (g *Gemini) Caption(ctx context.Context, imagePath string) (string, error) {
    if g.client == nil {
        return "", nil
    }
    b, err := os.ReadFile(imagePath)
    if err != nil || len(b) == 0 {
        return "", nil
    }
    mt := mime.TypeByExtension(filepath.Ext(imagePath))
    if mt == "" {
        // best-effort default
        mt = "image/png"
    }
    // Construct a multimodal prompt with inline image bytes.
    prompt := &genai.Content{
        Role: genai.RoleUser,
        Parts: []*genai.Part{
            {Text: "Describe this image for alt text in <= 12 words, factual, no embellishment."},
            {InlineData: &genai.Blob{MIMEType: mt, Data: b}},
        },
    }
    res, err := g.client.Models.GenerateContent(ctx, g.model, []*genai.Content{prompt}, nil)
    if err != nil {
        return "", nil
    }
    return res.Text(), nil
}

// ExtractStructure asks Gemini to parse a PDF and return structured sections with numbers, titles, page ranges, and text.
// The model should be gemini-2.5-pro for best results.
func (g *Gemini) ExtractStructure(ctx context.Context, pdfPath string, maxDepth, tocPages int) (StructuredDoc, error) {
    var out StructuredDoc
    if g.client == nil {
        return out, errors.New("gemini not configured")
    }
    b, err := os.ReadFile(pdfPath)
    if err != nil {
        return out, err
    }
    // Prompt the model to return strict JSON with a known schema.
    prompt := `You are a documentation parser. Return ONLY valid JSON - no markdown code blocks, no explanations.

Extract hierarchical table of contents and section texts from this PDF.
Output ONLY this JSON structure (no ` + "```json or ```markdown" + ` wrappers):
{
  "sections": [
    {"number": "1", "title": "Safety", "start_page": 4, "end_page": 6, "depth": 1, "text": "Full text content for section 1 with subsections as markdown headers...", "children": []}
  ]
}

CRITICAL RULES:
- number: Use exact section numbering from PDF (1, 1.1, 1.2.1, etc.)
- title: Section title without the number
- depth: 1 for top-level (1, 2, 3), 2 for subsections (1.1, 2.1), 3 for sub-subsections (1.2.1)
- start_page/end_page: 1-based page numbers for this section
- text: COMPLETE markdown text for the ENTIRE section including ALL subsections
  * For depth-1 sections, include ALL subsection content formatted with markdown headers
  * Use ## for depth-2 subsections (e.g., "## 1.1 General safety rules")
  * Use ### for depth-3 subsections (e.g., "### 1.2.1 Intended use")
  * Use #### for depth-4 subsections
  * Extract full paragraph text, preserve formatting, include all details
- children: LEAVE EMPTY [] - all content goes in the parent's "text" field
- DO NOT wrap response in code blocks
- Return ONLY the JSON object
`
    content := []*genai.Content{
        {
            Role: genai.RoleUser,
            Parts: []*genai.Part{
                {Text: prompt},
                {InlineData: &genai.Blob{MIMEType: "application/pdf", Data: b}},
            },
        },
    }
    res, err := g.client.Models.GenerateContent(ctx, g.model, content, nil)
    if err != nil {
        return out, fmt.Errorf("gemini API call failed: %w", err)
    }
    js := res.Text()

    // Debug: show what Gemini returned
    fmt.Printf("📄 Gemini response length: %d bytes\n", len(js))
    if len(js) > 500 {
        fmt.Printf("📄 Response preview: %s...\n", js[:500])
    } else {
        fmt.Printf("📄 Full response: %s\n", js)
    }

    // Strip code fences if present
    js = stripCodeFences(js)

    // Parse JSON
    if err := json.Unmarshal([]byte(js), &out); err != nil {
        // Try to find first JSON object in the text
        if s := findFirstJSON(js); s != "" {
            if err2 := json.Unmarshal([]byte(s), &out); err2 != nil {
                return out, fmt.Errorf("failed to parse Gemini response as JSON: %w (original error: %v)", err2, err)
            }
        } else {
            return out, fmt.Errorf("failed to parse Gemini response - no JSON found: %w", err)
        }
    }

    fmt.Printf("✅ Successfully parsed %d sections from Gemini response\n", len(out.Sections))
    return out, nil
}

func stripCodeFences(s string) string {
    // Remove markdown/json code fences like ```json, ```markdown, ```
    s = strings.TrimSpace(s)

    // Check for opening code fence
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

func findFirstJSON(s string) string {
    // naive scan for the first '{' to last '}'
    start := -1
    depth := 0
    for i, r := range s {
        if r == '{' {
            if start == -1 {
                start = i
            }
            depth++
        } else if r == '}' {
            if start != -1 {
                depth--
                if depth == 0 {
                    return s[start : i+1]
                }
            }
        }
    }
    return ""
}

func joinLines(s []string) string {
    if len(s) == 0 {
        return ""
    }
    out := s[0]
    for i := 1; i < len(s); i++ {
        out += "\n" + s[i]
    }
    return out
}

func splitLines(s string) []string {
    var lines []string
    cur := ""
    for _, r := range s {
        if r == '\n' || r == '\r' {
            if cur != "" {
                lines = append(lines, cur)
                cur = ""
            }
            continue
        }
        cur += string(r)
    }
    if cur != "" {
        lines = append(lines, cur)
    }
    return lines
}
