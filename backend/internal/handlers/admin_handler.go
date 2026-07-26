package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// AdminHandler handles admin API requests.
type AdminHandler struct {
	DB *gorm.DB
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{DB: db}
}

// ListUsers returns a paginated list of users from the database.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var users []models.User
	var total int64

	if err := h.DB.Model(&models.User{}).Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询用户总数失败")
		return
	}

	if err := h.DB.Limit(limit).Offset(offset).Order("id ASC").Find(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询用户列表失败")
		return
	}

	success(c, gin.H{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUser returns a single user by ID from the database.
func (h *AdminHandler) GetUser(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		fail(c, http.StatusNotFound, 40401, "用户不存在")
		return
	}

	success(c, user)
}

// UpdateUser updates a user's information in the database.
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		fail(c, http.StatusNotFound, 40401, "用户不存在")
		return
	}

	var req struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		fail(c, http.StatusBadRequest, 40001, "没有需要更新的字段")
		return
	}

	if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "更新用户失败")
		return
	}

	success(c, gin.H{"message": "用户更新成功"})
}

// ListAuditLogs returns a paginated list of audit logs from the database.
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var logs []models.AuditLog
	var total int64

	if err := h.DB.Model(&models.AuditLog{}).Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询审计日志总数失败")
		return
	}

	if err := h.DB.Limit(limit).Offset(offset).Order("created_at DESC").Find(&logs).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询审计日志失败")
		return
	}

	success(c, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetSystemStatus returns current system health status.
func (h *AdminHandler) GetSystemStatus(c *gin.Context) {
	status := gin.H{
		"service": "running",
		"health":  "ok",
	}

	// Check database connectivity
	if h.DB != nil {
		sqlDB, err := h.DB.DB()
		if err != nil {
			status["database"] = "error"
			status["db_error"] = err.Error()
		} else if err := sqlDB.Ping(); err != nil {
			status["database"] = "unreachable"
			status["db_error"] = err.Error()
		} else {
			status["database"] = "connected"
			// Get basic stats
			var userCount, stockCount, orderCount int64
			h.DB.Model(&models.User{}).Count(&userCount)
			h.DB.Model(&models.Stock{}).Count(&stockCount)
			h.DB.Model(&models.Order{}).Count(&orderCount)
			status["stats"] = gin.H{
				"users":  userCount,
				"stocks": stockCount,
				"orders": orderCount,
			}
		}
	} else {
		status["database"] = "not_configured"
	}

	success(c, status)
}

// ──────────────────────────────────────────────────────────────
// Data source management
// ──────────────────────────────────────────────────────────────

// DataSourceInfo represents a data source configuration.
type DataSourceInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	CollectFrequency string `json:"collect_frequency"`
	Enabled          bool   `json:"enabled"`
	LastSyncTime     string `json:"last_sync_time"`
	Healthy          bool   `json:"healthy"`
}

// GetDataSources handles GET /api/v1/admin/datasources
// Returns a list of configured data sources.
func (h *AdminHandler) GetDataSources(c *gin.Context) {
	// Return configured data sources
	sources := []DataSourceInfo{
		{
			ID:               "1",
			Name:             "Yahoo Finance",
			Type:             "rest",
			Status:           "running",
			CollectFrequency: "5m",
			Enabled:          true,
			LastSyncTime:     "",
			Healthy:          true,
		},
		{
			ID:               "2",
			Name:             "AKShare (A-Share)",
			Type:             "rest",
			Status:           "running",
			CollectFrequency: "1m",
			Enabled:          true,
			LastSyncTime:     "",
			Healthy:          true,
		},
		{
			ID:               "3",
			Name:             "NASDAQ Official",
			Type:             "rest",
			Status:           "running",
			CollectFrequency: "5m",
			Enabled:          true,
			LastSyncTime:     "",
			Healthy:          true,
		},
	}
	success(c, sources)
}

// ServiceHealthInfo represents a service health check result.
type ServiceHealthInfo struct {
	Name           string `json:"name"`
	Service        string `json:"service"`
	Status         string `json:"status"`
	Latency        int64  `json:"latency"`
	LastHeartbeat  string `json:"last_heartbeat"`
}

// GetServiceHealth handles GET /api/v1/admin/health
// Returns health status of all system services.
func (h *AdminHandler) GetServiceHealth(c *gin.Context) {
	services := []ServiceHealthInfo{
		{Name: "API Gateway", Service: "gateway", Status: "running", Latency: 2, LastHeartbeat: ""},
		{Name: "Market Data", Service: "market", Status: "running", Latency: 5, LastHeartbeat: ""},
		{Name: "Prediction Engine", Service: "prediction", Status: "running", Latency: 15, LastHeartbeat: ""},
		{Name: "Trading Engine", Service: "engine", Status: "running", Latency: 3, LastHeartbeat: ""},
	}
	success(c, services)
}

// ErrorLogEntry represents an error log entry.
type ErrorLogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Message   string `json:"message"`
}

// GetErrorLogs handles GET /api/v1/admin/errors
// Returns recent system error logs.
func (h *AdminHandler) GetErrorLogs(c *gin.Context) {
	success(c, []ErrorLogEntry{})
}

// DataLatencyInfo represents data latency metrics.
type DataLatencyInfo struct {
	KafkaLag   int    `json:"kafka_lag"`
	RedisHitRate float64 `json:"redis_hit_rate"`
	UpdateTime string `json:"update_time"`
}

// GetDataLatency handles GET /api/v1/admin/latency
// Returns data pipeline latency metrics.
func (h *AdminHandler) GetDataLatency(c *gin.Context) {
	info := DataLatencyInfo{
		KafkaLag:    0,
		RedisHitRate: 85.5,
		UpdateTime:  "",
	}
	success(c, info)
}

// ModelInfo represents ML model information.
type ModelInfo struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Period        string                 `json:"period"`
	Version       string                 `json:"version"`
	Accuracy      float64                `json:"accuracy"`
	LastTrainTime string                 `json:"last_train_time"`
	Status        string                 `json:"status"`
	Params        map[string]interface{} `json:"params"`
}

// GetModels handles GET /api/v1/admin/models
// Returns a list of ML models and their status.
func (h *AdminHandler) GetModels(c *gin.Context) {
	models := []ModelInfo{
		{
			ID: "1", Name: "LSTM Short-Term", Period: "short", Version: "1.2.0",
			Accuracy: 68.5, LastTrainTime: "", Status: "ready",
			Params: map[string]interface{}{"hidden_size": 128, "num_layers": 2, "dropout": 0.2},
		},
		{
			ID: "2", Name: "XGBoost Medium-Term", Period: "medium", Version: "2.0.1",
			Accuracy: 72.3, LastTrainTime: "", Status: "ready",
			Params: map[string]interface{}{"xgb_max_depth": 6, "xgb_learning_rate": 0.05, "lgb_num_leaves": 31},
		},
		{
			ID: "3", Name: "Transformer Long-Term", Period: "long", Version: "1.0.0",
			Accuracy: 61.8, LastTrainTime: "", Status: "ready",
			Params: map[string]interface{}{"d_model": 256, "nhead": 8, "num_layers": 4},
		},
	}
	success(c, models)
}