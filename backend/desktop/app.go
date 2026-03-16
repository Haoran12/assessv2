package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	mu sync.RWMutex

	api                http.Handler
	sqlDB              *sql.DB
	closeGuardEnabled  bool
	skipCloseGuardOnce bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.closeGuardEnabled = false
	a.skipCloseGuardOnce = false
	a.mu.Unlock()

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

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	a.ctx = ctx

	if a.skipCloseGuardOnce {
		a.skipCloseGuardOnce = false
		a.mu.Unlock()
		return false
	}

	needsConfirm := a.closeGuardEnabled
	a.mu.Unlock()

	if !needsConfirm {
		return false
	}

	selection, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.WarningDialog,
		Title:         "退出提醒",
		Message:       "检测到存在未保存的数据，退出后将丢失，是否继续退出？",
		Buttons:       []string{"继续退出", "取消"},
		DefaultButton: "取消",
		CancelButton:  "取消",
	})
	if err != nil {
		log.Printf("show close confirmation dialog failed: %v", err)
		return true
	}

	return selection != "继续退出"
}

func (a *App) onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()

	log.Printf(
		"second instance launch blocked; args=%v workingDirectory=%s",
		secondInstanceData.Args,
		secondInstanceData.WorkingDirectory,
	)

	if ctx == nil {
		return
	}

	runtime.WindowUnminimise(ctx)
	runtime.Show(ctx)
	if _, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "AssessV2",
		Message: "系统已在运行，已为你切换到现有窗口。",
	}); err != nil {
		log.Printf("show single-instance message dialog failed: %v", err)
	}
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

	if err := cleanupDescendantProcesses(uint32(os.Getpid())); err != nil {
		log.Printf("cleanup descendant processes failed: %v", err)
	}
}

func (a *App) ExitSystem() {
	a.mu.Lock()
	ctx := a.ctx
	if ctx != nil {
		a.skipCloseGuardOnce = true
	}
	a.mu.Unlock()

	if ctx == nil {
		return
	}

	if err := cleanupDescendantProcesses(uint32(os.Getpid())); err != nil {
		log.Printf("cleanup descendant processes before quit failed: %v", err)
	}

	runtime.Quit(ctx)
}

func (a *App) SetCloseGuard(enabled bool) {
	a.mu.Lock()
	a.closeGuardEnabled = enabled
	a.mu.Unlock()
}

func (a *App) SetPreferredDataYear(year int) error {
	if err := persistPreferredDataYear(year); err != nil {
		log.Printf("persist preferred assessment year failed: %v", err)
		return err
	}
	return nil
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
