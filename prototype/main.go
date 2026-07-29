// Command prototype demonstrates the Issue #1 closed-loop validation of the
// rule-fingerprint crawl engine: load a fingerprint -> fetch -> persist +
// download via the EXISTING pipeline (FileRepo + FileService) -> serve the
// supply-layer endpoint. It proves "collect -> store -> serve" composes.
//
// Build/run (Linux; the service layer uses Unix syscalls):
//
//	go run ./prototype -spec prototype/fingerprints/github_releases.yaml \
//	    -base ./tmp-prototype-data
//
// This is throwaway validation tooling, not shipped with the server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/rulesrc"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"github.com/Mi-Bee-Studio/mibeehive/internal/supply"
)

func main() {
	specPath := flag.String("spec", "prototype/fingerprints/github_releases.yaml", "path to a fingerprint YAML")
	base := flag.String("base", "./tmp-prototype-data", "storage base path")
	addr := flag.String("addr", "127.0.0.1:9099", "demo server address")
	dryRun := flag.Bool("dry-run", true, "if true, do not download file bodies (still writes index)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 1. Fresh SQLite + storage dir for an isolated run.
	dbPath := filepath.Join(*base, "proto.db")
	_ = os.MkdirAll(*base, 0o755)
	_ = os.Remove(dbPath)
	database, err := db.Open(dbPath)
	must(err, "open db")
	defer database.Close()
	must(db.Migrate(database), "migrate")

	// 2. A placeholder project row so files have a project_id.
	// source_type is CHECK-constrained to github/go/hashicorp/grafana in the
	// schema today; use "github" as a placeholder (the rule engine itself is
	// source-type-agnostic — see REPORT.md on the schema constraint).
	projRepo := db.NewProjectRepo(database)
	proj, err := projRepo.Create(context.Background(), "prototype", "Prototype", "github", "")
	must(err, "create project")
	projID := proj.ID

	// 3. Load fingerprint + fetch assets via the rule engine.
	spec, err := rulesrc.LoadSpec(*specPath)
	must(err, "load spec")
	fetcher := rulesrc.NewFetcher()
	assets, err := fetcher.Fetch(context.Background(), spec)
	must(err, "fetch")
	assets = rulesrc.ApplyFilter(assets, spec)
	logger.Info("fetched assets", "spec", spec.Name, "count", len(assets))

	// 4. Persist + (optionally) download via the EXISTING pipeline.
	resolver := service.NewStorageResolver(nil) // nil config -> uses base defaults; prototype sets paths manually
	fileRepo := db.NewFileRepo(database)
	var fileSvc *service.FileService
	if !*dryRun {
		fileSvc = service.NewFileService(database, resolver, 2, nil)
	}
	for _, a := range assets {
		localPath := filepath.Join(*base, spec.Name, a.Version, a.Filename)
		row := &db.File{
			ProjectID: projID, Version: a.Version, Filename: a.Filename,
			OS: a.OS, Arch: a.Arch, Ext: a.Ext, SizeBytes: a.SizeBytes,
			DownloadURL: a.DownloadURL, LocalPath: localPath, Checksum: a.Checksum,
			Status: string(model.FileStatusPending),
		}
		id, err := fileRepo.Create(context.Background(), row)
		if err != nil {
			logger.Warn("create file row", "file", a.Filename, "error", err)
			continue
		}
		if !*dryRun {
			mFile := &model.File{ID: id, ProjectID: int(projID), Filename: a.Filename,
				DownloadURL: a.DownloadURL, LocalPath: localPath, SizeBytes: a.SizeBytes,
				Status: model.FileStatusPending}
			if err := fileSvc.DownloadFile(context.Background(), mFile); err != nil {
				logger.Warn("download file", "file", a.Filename, "error", err)
			}
		}
	}

	// 5. Wire the supply endpoint (reuse FileService.StreamFile).
	if fileSvc == nil {
		fileSvc = service.NewFileService(database, resolver, 2, nil)
	}
	sh := supply.NewHandler(fileRepo, fileSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repo/index", sh.ServeIndex)
	mux.HandleFunc("GET /repo/files/{id}", sh.ServeFile)

	logger.Info("closed loop ready", "addr", *addr, "dryRun", *dryRun)
	fmt.Printf("\n  curl http://%s/repo/index | jq\n\n", *addr)
	if *dryRun {
		fmt.Println("  (dry-run: file bodies not downloaded; index lists pending rows)")
	}

	// Print a sample of the index for immediate, server-free inspection.
	files, _ := fileRepo.ListComplete(context.Background(), projID)
	if len(files) == 0 {
		// dry-run leaves status pending; show those instead for visibility
		files, _ = fileRepo.ListByProject(context.Background(), projID)
	}
	sample := make([]map[string]any, 0, len(files))
	for _, f := range files {
		sample = append(sample, map[string]any{"id": f.ID, "name": f.Filename, "version": f.Version, "os": f.OS, "arch": f.Arch})
	}
	b, _ := json.MarshalIndent(sample, "", "  ")
	fmt.Println("  sample rows:\n" + string(b))

	logger.Info("starting demo server (Ctrl+C to stop)")
	if err := http.ListenAndServe(*addr, mux); err != nil {
		must(err, "server")
	}
}

func must(err error, what string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", what, err))
	}
}
