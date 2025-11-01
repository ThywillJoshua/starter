package convert

import (
    "os"

    rpdf "rsc.io/pdf"
)

func extractTextPerPage(path string) ([]string, error) {
    // When using AI-exclusive mode, we don't need text extraction
    // Just return empty pages based on PDF page count
    n := fallbackPageCount(path)
    if n <= 0 {
        return []string{""}, nil
    }
    pages := make([]string, n)
    for i := range pages {
        pages[i] = ""
    }
    return pages, nil
}

func extractImages(path, outDir string) (int, error) {
    // When using Gemini-exclusive mode, image extraction is not needed
    // Images are handled by Gemini's multimodal capabilities
    if err := os.MkdirAll(outDir, 0o755); err != nil {
        return 0, err
    }
    return 0, nil
}

func fallbackPageCount(path string) int {
    f, err := os.Open(path)
    if err != nil {
        return 0
    }
    defer f.Close()
    doc, err := rpdf.NewReader(f, 0)
    if err != nil {
        return 0
    }
    return doc.NumPage()
}
