package ai

import "context"

type StructuredSection struct {
    Number   string              `json:"number"`
    Title    string              `json:"title"`
    Start    int                 `json:"start_page"`
    End      int                 `json:"end_page"`
    Depth    int                 `json:"depth"`
    Text     string              `json:"text"`
    Children []StructuredSection `json:"children,omitempty"`
}

type StructuredDoc struct {
    Sections []StructuredSection `json:"sections"`
}

type Enhancer interface {
    RepairToC(ctx context.Context, raw []string) ([]string, error)
    Summarize(ctx context.Context, text string, maxTokens int) (string, error)
    SuggestComponents(ctx context.Context, text string, allow []string, mode string) (string, error)
    Caption(ctx context.Context, imagePath string) (string, error)
    ExtractStructure(ctx context.Context, pdfPath string, maxDepth, tocPages int) (StructuredDoc, error)
}

type Noop struct{}

func (Noop) RepairToC(ctx context.Context, raw []string) ([]string, error) { return raw, nil }
func (Noop) Summarize(ctx context.Context, text string, maxTokens int) (string, error) {
    return "", nil
}
func (Noop) SuggestComponents(ctx context.Context, text string, allow []string, mode string) (string, error) {
    return "", nil
}
func (Noop) Caption(ctx context.Context, imagePath string) (string, error) { return "", nil }
func (Noop) ExtractStructure(ctx context.Context, pdfPath string, maxDepth, tocPages int) (StructuredDoc, error) {
    return StructuredDoc{}, nil
}

