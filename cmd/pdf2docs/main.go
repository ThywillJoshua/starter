package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

func main() {
    root := &cobra.Command{
        Use:   "pdf2docs",
        Short: "Convert a PDF into a Mintlify docs project",
    }

    root.AddCommand(convertCmd())

    if err := root.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

