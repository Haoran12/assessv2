package router

import (
	"net/http"

	"assessv2/backend/internal/api/handler"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	_ = db // DB will be injected into module handlers as features are implemented.

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(cfg.JWTSecret)
	moduleHandler := handler.NewModuleHandler()

	r.GET("/health", healthHandler.Health)

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
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
		system.Use(middleware.RequireJWT(cfg.JWTSecret))
		system.GET("/_ping", moduleHandler.ModulePing("system"))
		system.GET("/profile", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"username": "admin",
				},
			})
		})
	}

	return r
}

func registerModule(api *gin.RouterGroup, moduleName string, moduleHandler *handler.ModuleHandler) {
	group := api.Group("/" + moduleName)
	group.GET("/_ping", moduleHandler.ModulePing(moduleName))
}
