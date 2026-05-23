package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/garrettladley/pkgsite-mcp/internal/httpserver"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsiteapi"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		if err := serve(os.Args[1:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	switch os.Args[1] {
	case "serve":
		if err := serve(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "version":
		fmt.Println(version.CommandOutput())
	case "fetch-openapi":
		if err := fetchOpenAPI(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: pkgsite-mcp [serve|version|fetch-openapi]\n")
		os.Exit(2)
	}
}

func fetchOpenAPI(args []string) error {
	fs := flag.NewFlagSet("fetch-openapi", flag.ExitOnError)
	url := fs.String("url", pkgsiteapi.DefaultOpenAPIURL, "OpenAPI document URL")
	output := fs.String("output", pkgsiteapi.DefaultOpenAPIOutput, "output JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	patched, err := pkgsiteapi.FetchOpenAPI(context.Background(), nil, *url)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(*output); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}
	if err := os.WriteFile(*output, patched, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *output, err)
	}
	return nil
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	transport := fs.String("transport", "stdio", "transport to use: stdio or http")
	addr := fs.String("addr", "", "HTTP listen address; defaults to :$PORT or :8080")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch *transport {
	case "stdio":
		return mcpserver.RunStdio(context.Background())
	case "http":
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		cfg, err := httpserver.ConfigFromEnv(*addr)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}
		return httpserver.Run(context.Background(), cfg, logger)
	default:
		return fmt.Errorf("unsupported transport %q; expected stdio or http", *transport)
	}
}
