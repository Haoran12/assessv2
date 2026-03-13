package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	mu    sync.RWMutex
	api   http.Handler
	sqlDB *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, sqlDB, engine, err := bootstrapBackend()
	if err != nil {
		log.Printf("desktop backend bootstrap failed: %v", err)
		runtime.Quit(ctx)
		return
	}

	a.mu.Lock()
	a.api = engine
	a.sqlDB = sqlDB
	a.mu.Unlock()

	log.Printf("desktop backend ready at %s", cfg.Server.Address())
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sqlDB != nil {
		if err := a.sqlDB.Close(); err != nil {
			log.Printf("close desktop sqlite failed: %v", err)
		}
		a.sqlDB = nil
	}
	a.api = nil
}

func (a *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !isBackendRequest(request.URL.Path) {
		http.NotFound(writer, request)
		return
	}

	a.mu.RLock()
	handler := a.api
	a.mu.RUnlock()

	if handler == nil {
		http.Error(writer, "desktop backend is not ready", http.StatusServiceUnavailable)
		return
	}

	handler.ServeHTTP(writer, request)
}

func isBackendRequest(path string) bool {
	if path == "/api" || path == "/health" {
		return true
	}
	return strings.HasPrefix(path, "/api/")
}
