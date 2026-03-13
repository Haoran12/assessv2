package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	assetDir, err := resolveFrontendDistDir()
	if err != nil {
		log.Fatal(err)
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:  "AssessV2",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets:  os.DirFS(assetDir),
			Handler: app,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
	})
	if err != nil {
		log.Fatal(err)
	}
}

func resolveFrontendDistDir() (string, error) {
	if envDist := os.Getenv("ASSESS_FRONTEND_DIST"); envDist != "" {
		if dir, ok := existingDir(envDist); ok {
			return dir, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)

	candidates := []string{
		filepath.Join(exeDir, "frontend", "dist"),
		filepath.Join(exeDir, "dist"),
		filepath.Join(cwd, "..", "..", "frontend", "dist"),
		filepath.Join(cwd, "..", "frontend", "dist"),
		filepath.Join(cwd, "frontend", "dist"),
	}

	for _, candidate := range candidates {
		if dir, ok := existingDir(candidate); ok {
			return dir, nil
		}
	}

	return "", fmt.Errorf("frontend dist directory not found; run `npm --prefix frontend run build` or set ASSESS_FRONTEND_DIST")
}

func existingDir(path string) (string, bool) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return resolved, true
}
