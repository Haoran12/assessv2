package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	mu sync.RWMutex

	api                    http.Handler
	sqlDBs                 []*sql.DB
	closeGuardEnabled      bool
	skipCloseGuardOnce     bool
	desktopFirstUse        bool
	desktopInitialPassword string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.closeGuardEnabled = false
	a.skipCloseGuardOnce = false
	a.desktopFirstUse = false
	a.desktopInitialPassword = ""
	a.mu.Unlock()

	firstUse, firstUseErr := detectDesktopFirstUse()
	if firstUseErr != nil {
		log.Printf("detect desktop first-use state failed: %v", firstUseErr)
	}

	cfg, sqlDBs, engine, err := bootstrapBackend()
	if err != nil {
		log.Printf("desktop backend bootstrap failed: %v", err)
		message := fmt.Sprintf(
			"The application failed to start.\n\nReason:\n%v\n\nPlease contact support with this message.",
			err,
		)
		if _, dialogErr := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:          runtime.ErrorDialog,
			Title:         "考核系统启动失败",
			Message:       message,
			Buttons:       []string{"OK"},
			DefaultButton: "OK",
			CancelButton:  "OK",
		}); dialogErr != nil {
			log.Printf("show startup error dialog failed: %v", dialogErr)
		}
		runtime.Quit(ctx)
		return
	}

	a.mu.Lock()
	a.api = engine
	a.sqlDBs = sqlDBs
	a.desktopFirstUse = firstUse && firstUseErr == nil
	if a.desktopFirstUse {
		a.desktopInitialPassword = cfg.DefaultPassword
	} else {
		a.desktopInitialPassword = ""
	}
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

	runtime.EventsEmit(ctx, "app:close-requested")
	return true
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
		Title:   "考核系统",
		Message: "The app is already running. Switched to the existing window.",
	}); err != nil {
		log.Printf("show single-instance message dialog failed: %v", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, item := range a.sqlDBs {
		if item == nil {
			continue
		}
		if err := item.Close(); err != nil {
			log.Printf("close desktop sqlite failed: %v", err)
		}
	}
	a.sqlDBs = nil
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
	return fmt.Errorf("preferred data year is deprecated in session-based mode")
}

type DesktopBootstrapInfo struct {
	IsFirstUse      bool   `json:"isFirstUse"`
	DefaultPassword string `json:"defaultPassword"`
}

func (a *App) GetDesktopBootstrapInfo() DesktopBootstrapInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return DesktopBootstrapInfo{
		IsFirstUse:      a.desktopFirstUse,
		DefaultPassword: a.desktopInitialPassword,
	}
}

func (a *App) SwitchToEnglishInputMethod() error {
	return switchToEnglishInputMethod()
}

func (a *App) SaveXlsxFileWithDialog(defaultFileName string, contentBase64 string) (string, error) {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return "", fmt.Errorf("desktop context is not ready")
	}

	fileName := strings.TrimSpace(defaultFileName)
	if fileName == "" {
		fileName = "assessment-export.xlsx"
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".xlsx") {
		fileName += ".xlsx"
	}

	targetPath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:           "导出考核结果",
		DefaultFilename: fileName,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Excel 文件 (*.xlsx)",
				Pattern:     "*.xlsx",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(targetPath), ".xlsx") {
		targetPath += ".xlsx"
	}

	contentRaw := strings.TrimSpace(contentBase64)
	if contentRaw == "" {
		return "", fmt.Errorf("export content is empty")
	}
	data, err := base64.StdEncoding.DecodeString(contentRaw)
	if err != nil {
		return "", fmt.Errorf("decode export content: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("export content is empty")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create export directory: %w", err)
	}
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return targetPath, nil
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
