package convert

import (
    "github.com/thywilljoshua/pdf-to-docs/internal/ai"
)

type Section struct {
    Number   string    `json:"number"`
    Title    string    `json:"title"`
    Start    int       `json:"start_page"`
    End      int       `json:"end_page"`
    Depth    int       `json:"depth"`
    Slug     string    `json:"slug"`
    Children []Section `json:"children,omitempty"`
}

type Result struct {
    Sections []Section `json:"sections"`
    Images   int       `json:"images_extracted"`
    OutDir   string    `json:"out_dir"`
}

type Config struct {
    OutDir        string
    KeepImages    bool
    UseToC        bool
    FallbackSplit string
    MaxDepth      int
    ToCPages      int
    SiteName      string
    SlugPrefix    string
    AIExclusive   bool
    Enhancer      ai.Enhancer
}

// Alias types from ai package for convenience
type StructuredDoc = ai.StructuredDoc
type StructuredSection = ai.StructuredSection
