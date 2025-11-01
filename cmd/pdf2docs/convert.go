package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thywilljoshua/pdf-to-docs/internal/ai"
	"github.com/thywilljoshua/pdf-to-docs/internal/convert"
)

func convertCmd() *cobra.Command {
    var out string
    var keepImages bool
    var useToC bool
    var fallback string
    var maxDepth int
    var tocPages int
    var aiProvider string
    var aiComponents string
    var aiAllow string
    var aiExclusive bool
    var siteName string
    var slugPrefix string

    cmd := &cobra.Command{
        Use:   "convert <pdf>",
        Short: "Convert a PDF into MDX pages and docs.json",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            pdfPath := args[0]
            if out == "" {
                out = filepath.Join(".")
            }

            var enhancer ai.Enhancer = ai.Noop{}
            if strings.EqualFold(aiProvider, "gemini") {
                ctx := context.Background()
                g, err := ai.NewGemini(ctx,"AIzaSyC1ZXkbXICCnEOwVX5VGz2tPSfcp7sflhs", "gemini-2.5-pro")
                if err == nil {
                    g.ComponentsMode = aiComponents
                    if aiAllow != "" {
                        g.ComponentsAllow = strings.Split(aiAllow, ",")
                    }
                    enhancer = g
                }
            }

            conf := convert.Config{
                OutDir:        out,
                KeepImages:    keepImages,
                UseToC:        useToC,
                FallbackSplit: fallback,
                MaxDepth:      maxDepth,
                ToCPages:      tocPages,
                SiteName:      siteName,
                SlugPrefix:    slugPrefix,
                Enhancer:      enhancer,
                AIExclusive:   aiExclusive,
            }

            res, err := convert.Run(cmd.Context(), pdfPath, conf)
            if err != nil {
                return err
            }
            b, _ := json.MarshalIndent(res, "", "  ")
            fmt.Fprintln(cmd.OutOrStdout(), string(b))
            return nil
        },
    }
    cmd.Flags().StringVarP(&out, "out", "o", "", "output directory for the docs (default: current directory)")
    cmd.Flags().BoolVar(&keepImages, "keep-images", true, "extract and embed images")
    cmd.Flags().BoolVar(&useToC, "toc", true, "use Table of Contents splitting when available")
    cmd.Flags().StringVar(&fallback, "fallback", "page", "fallback split when no ToC: page|heading")
    cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "maximum ToC depth to generate")
    cmd.Flags().StringVar(&aiProvider, "ai", "off", "AI provider: off|gemini")
    cmd.Flags().StringVar(&aiComponents, "ai-components", "conservative", "AI component mode: off|conservative|balanced|aggressive")
    cmd.Flags().StringVar(&aiAllow, "ai-components-allow", "callout,steps,accordion", "Comma-separated allowlist of components")
    cmd.Flags().BoolVar(&aiExclusive, "ai-exclusive", false, "Use Gemini exclusively to extract ToC + content from PDF (OCR included)")
    cmd.Flags().StringVar(&siteName, "site-name", "", "Override site name in docs.json (defaults to starter)")
    cmd.Flags().StringVar(&slugPrefix, "slug-prefix", "", "Optional slug prefix for generated pages")
    cmd.Flags().IntVar(&tocPages, "toc-pages", 16, "Scan up to N early pages for a multi-page Table of Contents")
    return cmd
}
