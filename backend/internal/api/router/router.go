package router

import (
	"assessv2/backend/internal/api/handler"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/repository"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	healthHandler := handler.NewHealthHandler()
	moduleHandler := handler.NewModuleHandler()
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	authService := service.NewAuthService(userRepo, auditRepo, cfg.JWTSecret, cfg.EnforceMustChangePassword)
	userService := service.NewUserService(userRepo, auditRepo, cfg.DefaultPassword)

	authHandler := handler.NewAuthHandler(authService)
	systemHandler := handler.NewSystemHandler(authService, userService)

	r.GET("/health", healthHandler.Health)

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.Use(middleware.RequireJWT(cfg.JWTSecret))
			auth.POST("/change-password", authHandler.ChangePassword)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/_ping", moduleHandler.ModulePing("auth"))
		}

		registerModule(api, "org", moduleHandler)
		registerModule(api, "assessment", moduleHandler)
		registerModule(api, "rules", moduleHandler)
		registerModule(api, "scores", moduleHandler)
		registerModule(api, "votes", moduleHandler)
		registerModule(api, "calc", moduleHandler)
		registerModule(api, "reports", moduleHandler)
		registerModule(api, "backup", moduleHandler)

		system := api.Group("/system")
		system.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		system.GET("/_ping", moduleHandler.ModulePing("system"))
		system.GET("/profile", systemHandler.Profile)
		system.GET("/users", middleware.RequirePermission("user:view"), systemHandler.ListUsers)
		system.POST("/users/:id/reset-password", middleware.RequirePermission("user:update"), systemHandler.ResetPassword)
		system.PUT("/users/:id/status", middleware.RequirePermission("user:update"), systemHandler.UpdateUserStatus)
	}

	return r
}

func registerModule(api *gin.RouterGroup, moduleName string, moduleHandler *handler.ModuleHandler) {
	group := api.Group("/" + moduleName)
	group.GET("/_ping", moduleHandler.ModulePing(moduleName))
}
