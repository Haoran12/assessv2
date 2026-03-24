package router

import (
	"assessv2/backend/internal/api/handler"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/repository"
	"assessv2/backend/internal/service"
	"context"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	return NewWithDatabases(cfg, db, db)
}

func NewWithDatabases(cfg config.Config, businessDB *gorm.DB, accountDB *gorm.DB) *gin.Engine {
	if accountDB == nil {
		accountDB = businessDB
	}

	r := gin.New()
	r.Use(middleware.RequestContext(), middleware.AccessLogger(), gin.Recovery())

	healthHandler := handler.NewHealthHandler()
	moduleHandler := handler.NewModuleHandler()
	userRepo := repository.NewUserRepository(accountDB)
	roleRepo := repository.NewRoleRepository(accountDB)
	userRoleRepo := repository.NewUserRoleRepository(accountDB)
	userOrgRepo := repository.NewUserOrganizationRepository(accountDB)
	accountAuditRepo := repository.NewAuditRepository(accountDB)
	businessAuditRepo := repository.NewAuditRepository(businessDB)
	authService := service.NewAuthService(userRepo, accountAuditRepo, cfg.JWTSecret, cfg.EnforceMustChangePassword)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, userOrgRepo, accountAuditRepo, cfg.DefaultPassword)
	orgService := service.NewOrgService(businessDB, businessAuditRepo)
	assessmentService := service.NewAssessmentSessionService(businessDB, businessAuditRepo)
	ruleService := service.NewRuleManagementService(businessDB, businessAuditRepo)
	settingsService := service.NewSystemSettingService(businessDB, businessAuditRepo)
	backupService := service.NewBackupService(businessDB, businessAuditRepo, cfg.Database.Path)
	auditService := service.NewAuditService(businessDB, accountDB, businessAuditRepo)
	backupService.StartAutoBackup(context.Background())

	authHandler := handler.NewAuthHandler(authService)
	systemHandler := handler.NewSystemHandler(authService, userService)
	orgHandler := handler.NewOrgHandler(orgService)
	assessmentHandler := handler.NewAssessmentHandler(assessmentService)
	ruleHandler := handler.NewRuleHandler(ruleService)
	backupHandler := handler.NewBackupHandler(backupService)
	settingsHandler := handler.NewSettingsHandler(settingsService)
	auditHandler := handler.NewAuditHandler(auditService)

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

		org := api.Group("/org")
		org.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		org.GET("/_ping", moduleHandler.ModulePing("org"))
		org.GET("/tree", middleware.RequirePermission("org:view"), orgHandler.Tree)
		org.GET("/organizations", middleware.RequirePermission("org:view"), orgHandler.ListOrganizations)
		org.POST("/organizations", middleware.RequirePermission("org:update"), orgHandler.CreateOrganization)
		org.PUT("/organizations/:id", middleware.RequirePermission("org:update"), orgHandler.UpdateOrganization)
		org.DELETE("/organizations/:id", middleware.RequireRoot(), orgHandler.DeleteOrganization)
		org.GET("/departments", middleware.RequirePermission("org:view"), orgHandler.ListDepartments)
		org.POST("/departments", middleware.RequirePermission("org:update"), orgHandler.CreateDepartment)
		org.PUT("/departments/:id", middleware.RequirePermission("org:update"), orgHandler.UpdateDepartment)
		org.DELETE("/departments/:id", middleware.RequireRoot(), orgHandler.DeleteDepartment)
		org.GET("/position-levels", middleware.RequirePermission("org:view"), orgHandler.ListPositionLevels)
		org.GET("/assessment-categories", middleware.RequirePermission("assessment:view"), orgHandler.ListAssessmentCategories)
		org.POST("/position-levels", middleware.RequireRoot(), orgHandler.CreatePositionLevel)
		org.PUT("/position-levels/:id", middleware.RequireRoot(), orgHandler.UpdatePositionLevel)
		org.DELETE("/position-levels/:id", middleware.RequireRoot(), orgHandler.DeletePositionLevel)
		org.GET("/employees", middleware.RequirePermission("org:view"), orgHandler.ListEmployees)
		org.POST("/employees", middleware.RequirePermission("org:update"), orgHandler.CreateEmployee)
		org.PUT("/employees/:id", middleware.RequirePermission("org:update"), orgHandler.UpdateEmployee)
		org.DELETE("/employees/:id", middleware.RequireRoot(), orgHandler.DeleteEmployee)
		org.POST("/employees/:id/transfer", middleware.RequirePermission("org:update"), orgHandler.TransferEmployee)
		org.GET("/employees/:id/history", middleware.RequirePermission("org:view"), orgHandler.EmployeeHistory)

		assessment := api.Group("/assessment")
		assessment.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		assessment.GET("/_ping", moduleHandler.ModulePing("assessment"))
		assessment.GET("/sessions", middleware.RequirePermission("assessment:view"), assessmentHandler.ListSessions)
		assessment.POST("/sessions", middleware.RequirePermission("assessment:update"), assessmentHandler.CreateSession)
		assessment.GET("/sessions/:id", middleware.RequirePermission("assessment:view"), assessmentHandler.GetSession)
		assessment.PUT("/sessions/:id", middleware.RequirePermission("assessment:update"), assessmentHandler.UpdateSession)
		assessment.PUT("/sessions/:id/status", middleware.RequirePermission("assessment:update"), assessmentHandler.UpdateSessionStatus)
		assessment.PUT("/sessions/:id/periods", middleware.RequirePermission("assessment:update"), assessmentHandler.ReplacePeriods)
		assessment.PUT("/sessions/:id/object-groups", middleware.RequirePermission("assessment:update"), assessmentHandler.ReplaceObjectGroups)
		assessment.GET("/sessions/:id/object-candidates", middleware.RequirePermission("assessment:view"), assessmentHandler.ListObjectCandidates)
		assessment.GET("/sessions/:id/objects", middleware.RequirePermission("assessment:view"), assessmentHandler.ListObjects)
		assessment.GET("/sessions/:id/calculated-objects", middleware.RequirePermission("assessment:view"), assessmentHandler.ListCalculatedObjects)
		assessment.PUT("/sessions/:id/module-scores", middleware.RequirePermission("assessment:update"), assessmentHandler.UpsertModuleScores)
		assessment.PUT("/sessions/:id/objects", middleware.RequirePermission("assessment:update"), assessmentHandler.ReplaceObjects)
		assessment.POST("/sessions/:id/objects/reset-default", middleware.RequirePermission("assessment:update"), assessmentHandler.ResetObjects)

		rules := api.Group("/rules")
		rules.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		rules.GET("/_ping", moduleHandler.ModulePing("rules"))
		rules.GET("/expression-context", middleware.RequirePermission("rule:view"), ruleHandler.GetExpressionContext)
		rules.GET("/files", middleware.RequirePermission("rule:view"), ruleHandler.ListRuleFiles)
		rules.POST("/files", middleware.RequirePermission("rule:update"), ruleHandler.CreateRuleFile)
		rules.PUT("/files/:id", middleware.RequirePermission("rule:update"), ruleHandler.UpdateRuleFile)
		rules.POST("/files/:id/dependency-check", middleware.RequirePermission("rule:update"), ruleHandler.CheckRuleDependencies)

		backup := api.Group("/backup")
		backup.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		backup.GET("/_ping", moduleHandler.ModulePing("backup"))
		backup.GET("/records", middleware.RequirePermission("backup:view"), backupHandler.List)
		backup.POST("/records", middleware.RequirePermission("backup:update"), backupHandler.Create)
		backup.GET("/records/:id/download", middleware.RequirePermission("backup:view"), backupHandler.Download)
		backup.DELETE("/records/:id", middleware.RequirePermission("backup:update"), backupHandler.Delete)
		backup.POST("/records/:id/restore", middleware.RequirePermission("backup:update"), backupHandler.Restore)
		backup.GET("/org-packages", middleware.RequirePermission("backup:org:view"), backupHandler.ListOrgPackages)
		backup.POST("/org-packages", middleware.RequirePermission("backup:org:update"), backupHandler.CreateOrgPackage)
		backup.GET("/org-packages/:id/download", middleware.RequirePermission("backup:org:view"), backupHandler.DownloadOrgPackage)
		backup.POST("/org-packages/:id/restore", middleware.RequirePermission("backup:org:update"), backupHandler.RestoreOrgPackage)

		system := api.Group("/system")
		system.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		system.GET("/_ping", moduleHandler.ModulePing("system"))
		system.GET("/profile", systemHandler.Profile)
		system.GET("/users", middleware.RequireRoot(), systemHandler.ListUsers)
		system.POST("/users", middleware.RequireRoot(), systemHandler.CreateUser)
		system.PUT("/users/:id", middleware.RequireRoot(), systemHandler.UpdateUser)
		system.DELETE("/users/:id", middleware.RequireRoot(), systemHandler.DeleteUser)
		system.POST("/users/:id/reset-password", middleware.RequireRoot(), systemHandler.ResetPassword)
		system.PUT("/users/:id/status", middleware.RequireRoot(), systemHandler.UpdateUserStatus)
		system.GET("/groups", middleware.RequireRoot(), systemHandler.ListUserGroups)
		system.POST("/groups", middleware.RequireRoot(), systemHandler.CreateUserGroup)
		system.PUT("/groups/:id", middleware.RequireRoot(), systemHandler.UpdateUserGroup)
		system.DELETE("/groups/:id", middleware.RequireRoot(), systemHandler.DeleteUserGroup)
		system.PUT("/users/:id/groups", middleware.RequireRoot(), systemHandler.UpdateUserGroups)
		system.GET("/settings", middleware.RequireRoot(), settingsHandler.List)
		system.PUT("/settings", middleware.RequireRoot(), settingsHandler.Update)
		system.GET("/audit-logs", middleware.RequirePermission("audit:view"), auditHandler.List)
		system.GET("/audit-logs/:id", middleware.RequirePermission("audit:view"), auditHandler.Detail)
		system.POST("/audit-logs/:id/rollback", middleware.RequirePermission("audit:rollback"), auditHandler.Rollback)
	}

	return r
}

func registerModule(api *gin.RouterGroup, moduleName string, moduleHandler *handler.ModuleHandler) {
	group := api.Group("/" + moduleName)
	group.GET("/_ping", moduleHandler.ModulePing(moduleName))
}
