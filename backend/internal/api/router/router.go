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
	accountAuditRepo := repository.NewAuditRepository(accountDB)
	businessAuditRepo := repository.NewAuditRepository(businessDB)
	authService := service.NewAuthService(userRepo, accountAuditRepo, cfg.JWTSecret, cfg.EnforceMustChangePassword)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, accountAuditRepo, cfg.DefaultPassword)
	orgService := service.NewOrgService(businessDB, businessAuditRepo)
	assessmentService := service.NewAssessmentService(businessDB, businessAuditRepo)
	ruleService := service.NewRuleService(businessDB, businessAuditRepo)
	calcService := service.NewCalculationService(businessDB, businessAuditRepo)
	scoreService := service.NewScoreService(businessDB, businessAuditRepo, calcService)
	importExportService := service.NewImportExportService(businessDB, businessAuditRepo, scoreService)
	voteService := service.NewVoteService(businessDB, accountDB, businessAuditRepo, calcService)
	settingsService := service.NewSystemSettingService(businessDB, businessAuditRepo)
	backupService := service.NewBackupService(businessDB, businessAuditRepo, cfg.Database.Path)
	auditService := service.NewAuditService(businessDB, businessAuditRepo)
	objectLinkService := service.NewAssessmentObjectUserLinkService(businessDB, businessAuditRepo)
	backupService.StartAutoBackup(context.Background())

	authHandler := handler.NewAuthHandler(authService)
	systemHandler := handler.NewSystemHandler(authService, userService, objectLinkService)
	orgHandler := handler.NewOrgHandler(orgService)
	assessmentHandler := handler.NewAssessmentHandler(assessmentService)
	ruleHandler := handler.NewRuleHandler(ruleService)
	scoreHandler := handler.NewScoreHandler(scoreService)
	importExportHandler := handler.NewImportExportHandler(importExportService)
	voteHandler := handler.NewVoteHandler(voteService)
	calcHandler := handler.NewCalcHandler(calcService)
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
		org.DELETE("/organizations/:id", middleware.RequirePermission("org:update"), orgHandler.DeleteOrganization)
		org.GET("/departments", middleware.RequirePermission("org:view"), orgHandler.ListDepartments)
		org.POST("/departments", middleware.RequirePermission("org:update"), orgHandler.CreateDepartment)
		org.PUT("/departments/:id", middleware.RequirePermission("org:update"), orgHandler.UpdateDepartment)
		org.DELETE("/departments/:id", middleware.RequirePermission("org:update"), orgHandler.DeleteDepartment)
		org.GET("/position-levels", middleware.RequirePermission("org:view"), orgHandler.ListPositionLevels)
		org.GET("/assessment-categories", middleware.RequirePermission("assessment:view"), orgHandler.ListAssessmentCategories)
		org.POST("/position-levels", middleware.RequireRoot(), orgHandler.CreatePositionLevel)
		org.PUT("/position-levels/:id", middleware.RequireRoot(), orgHandler.UpdatePositionLevel)
		org.DELETE("/position-levels/:id", middleware.RequireRoot(), orgHandler.DeletePositionLevel)
		org.GET("/employees", middleware.RequirePermission("org:view"), orgHandler.ListEmployees)
		org.POST("/employees", middleware.RequirePermission("org:update"), orgHandler.CreateEmployee)
		org.PUT("/employees/:id", middleware.RequirePermission("org:update"), orgHandler.UpdateEmployee)
		org.DELETE("/employees/:id", middleware.RequirePermission("org:update"), orgHandler.DeleteEmployee)
		org.POST("/employees/:id/transfer", middleware.RequirePermission("org:update"), orgHandler.TransferEmployee)
		org.GET("/employees/:id/history", middleware.RequirePermission("org:view"), orgHandler.EmployeeHistory)

		assessment := api.Group("/assessment")
		assessment.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		assessment.GET("/_ping", moduleHandler.ModulePing("assessment"))
		assessment.GET("/years", middleware.RequirePermission("assessment:view"), assessmentHandler.ListYears)
		assessment.POST("/years", middleware.RequirePermission("assessment:update"), assessmentHandler.CreateYear)
		assessment.PUT("/years/:id/status", middleware.RequirePermission("assessment:update"), assessmentHandler.UpdateYearStatus)
		assessment.GET("/years/:id/periods", middleware.RequirePermission("assessment:view"), assessmentHandler.ListPeriods)
		assessment.GET("/years/:id/objects", middleware.RequirePermission("assessment:view"), assessmentHandler.ListObjects)
		assessment.GET("/period-templates", middleware.RequirePermission("assessment:view"), assessmentHandler.ListPeriodTemplates)
		assessment.PUT("/period-templates", middleware.RequirePermission("assessment:update"), assessmentHandler.UpdatePeriodTemplates)
		assessment.PUT("/periods/:id/status", middleware.RequirePermission("assessment:update"), assessmentHandler.UpdatePeriodStatus)

		rules := api.Group("/rules")
		rules.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		rules.GET("/_ping", moduleHandler.ModulePing("rules"))
		rules.GET("", middleware.RequirePermission("rule:view"), ruleHandler.ListRules)
		rules.POST("", middleware.RequirePermission("rule:update"), ruleHandler.CreateRule)
		rules.PUT("/:id", middleware.RequirePermission("rule:update"), ruleHandler.UpdateRule)
		rules.GET("/templates", middleware.RequirePermission("rule:view"), ruleHandler.ListTemplates)
		rules.POST("/templates", middleware.RequirePermission("rule:update"), ruleHandler.CreateTemplate)
		rules.POST("/templates/:id/apply", middleware.RequirePermission("rule:update"), ruleHandler.ApplyTemplate)
		rules.GET("/bindings", middleware.RequirePermission("rule:view"), ruleHandler.ListBindings)
		rules.POST("/bindings", middleware.RequirePermission("rule:update"), ruleHandler.CreateBinding)
		rules.PUT("/bindings/:id", middleware.RequirePermission("rule:update"), ruleHandler.UpdateBinding)
		rules.DELETE("/bindings/:id", middleware.RequirePermission("rule:update"), ruleHandler.DeleteBinding)
		rules.GET("/:id", middleware.RequirePermission("rule:view"), ruleHandler.GetRule)
		rules.POST("/:id/templates", middleware.RequirePermission("rule:update"), ruleHandler.CreateTemplateFromRule)

		scores := api.Group("/scores")
		scores.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		scores.GET("/_ping", moduleHandler.ModulePing("scores"))
		scores.GET("/direct", middleware.RequirePermission("score:view"), scoreHandler.ListDirectScores)
		scores.POST("/direct", middleware.RequirePermission("score:update"), scoreHandler.CreateDirectScore)
		scores.POST("/direct/batch", middleware.RequirePermission("score:update"), scoreHandler.BatchUpsertDirectScores)
		scores.PUT("/direct/:id", middleware.RequirePermission("score:update"), scoreHandler.UpdateDirectScore)
		scores.DELETE("/direct/:id", middleware.RequirePermission("score:update"), scoreHandler.DeleteDirectScore)
		scores.GET("/import/templates/:type", middleware.RequirePermission("score:view"), importExportHandler.DownloadTemplate)
		scores.POST("/import/direct/preview", middleware.RequirePermission("score:update"), importExportHandler.PreviewDirectScoreImport)
		scores.POST("/import/direct/confirm", middleware.RequirePermission("score:update"), importExportHandler.ConfirmDirectScoreImport)
		scores.GET("/extra", middleware.RequirePermission("score:view"), scoreHandler.ListExtraPoints)
		scores.POST("/extra", middleware.RequirePermission("score:update"), scoreHandler.CreateExtraPoint)
		scores.PUT("/extra/:id", middleware.RequirePermission("score:update"), scoreHandler.UpdateExtraPoint)
		scores.POST("/extra/:id/approve", middleware.RequirePermission("score:update"), scoreHandler.ApproveExtraPoint)
		scores.DELETE("/extra/:id", middleware.RequirePermission("score:update"), scoreHandler.DeleteExtraPoint)

		votes := api.Group("/votes")
		votes.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		votes.GET("/_ping", moduleHandler.ModulePing("votes"))
		votes.POST("/tasks/generate", middleware.RequirePermission("vote:manage"), voteHandler.GenerateVoteTasks)
		votes.GET("/tasks", middleware.RequirePermission("vote:view"), voteHandler.ListVoteTasks)
		votes.POST("/tasks/:id/draft", middleware.RequirePermission("vote:submit"), voteHandler.SaveVoteDraft)
		votes.POST("/tasks/:id/submit", middleware.RequirePermission("vote:submit"), voteHandler.SubmitVote)
		votes.POST("/tasks/:id/reset", middleware.RequireRoot(), voteHandler.ResetVoteTask)
		votes.GET("/statistics", middleware.RequirePermission("vote:detail:view"), voteHandler.VoteStatistics)

		calc := api.Group("/calc")
		calc.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		calc.GET("/_ping", moduleHandler.ModulePing("calc"))
		calc.POST("/recalculate", middleware.RequirePermission("score:update"), calcHandler.Recalculate)
		calc.GET("/scores", middleware.RequirePermission("score:view"), calcHandler.ListCalculatedScores)
		calc.GET("/scores/:id/modules", middleware.RequirePermission("score:view"), calcHandler.ListCalculatedModuleScores)
		calc.GET("/rankings", middleware.RequirePermission("score:view"), calcHandler.ListRankings)

		reports := api.Group("/reports")
		reports.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		reports.GET("/_ping", moduleHandler.ModulePing("reports"))
		reports.GET("/export", middleware.RequirePermission("report:view"), importExportHandler.ExportWorkbook)

		backup := api.Group("/backup")
		backup.Use(middleware.RequireJWT(cfg.JWTSecret), middleware.RequireOrgScope())
		backup.GET("/_ping", moduleHandler.ModulePing("backup"))
		backup.GET("/records", middleware.RequirePermission("backup:view"), backupHandler.List)
		backup.POST("/records", middleware.RequirePermission("backup:update"), backupHandler.Create)
		backup.GET("/records/:id/download", middleware.RequirePermission("backup:view"), backupHandler.Download)
		backup.DELETE("/records/:id", middleware.RequirePermission("backup:update"), backupHandler.Delete)
		backup.POST("/records/:id/restore", middleware.RequirePermission("backup:update"), backupHandler.Restore)

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
		system.GET("/users/:id/object-links", middleware.RequireRoot(), systemHandler.ListUserObjectLinks)
		system.PUT("/users/:id/object-links", middleware.RequireRoot(), systemHandler.ReplaceUserObjectLinks)
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
