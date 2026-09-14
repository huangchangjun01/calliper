package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/models"
	"github.com/quant-trading/backend/internal/services"
	"github.com/quant-trading/backend/internal/util"
)

// AdminUserDTO is the normalized user representation returned to the frontend.
type AdminUserDTO struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// toAdminUserDTO converts a models.User into the normalized admin DTO.
func toAdminUserDTO(u models.User) AdminUserDTO {
	status := "active"
	if !u.IsActive {
		status = "disabled"
	}
	return AdminUserDTO{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		Status:    status,
		CreatedAt: u.CreatedAt,
	}
}

// AdminHandler handles admin API requests.
type AdminHandler struct {
	DB *gorm.DB

	// rdb is the Redis client (may be nil when Redis is unavailable).
	rdb *redis.Client
	// tsdb is the TimescaleDB connection used for time-series price data
	// (may be nil when the TSDB connection is unavailable).
	tsdb *gorm.DB

	// stockService syncs stock lists (used by the sina data source).
	stockService *services.StockService
	// marketService collects real-time market data (used by tencent/eastmoney).
	marketService *services.MarketDataService

	// ML service proxy config (baseURL like http://localhost:8000).
	mlBaseURL string
	mlAPIKey  string
	// mlClient is used for short-lived GET/POST proxy calls.
	mlClient *http.Client
	// trainClient uses a longer timeout because training can be slow.
	trainClient *http.Client
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(db *gorm.DB, rdb *redis.Client, tsdb *gorm.DB, stockSvc *services.StockService, marketSvc *services.MarketDataService) *AdminHandler {
	host := os.Getenv("ML_SERVICE_HOST")
	if host == "" {
		host = "localhost"
	}
	return &AdminHandler{
		DB:            db,
		rdb:           rdb,
		tsdb:          tsdb,
		stockService:  stockSvc,
		marketService: marketSvc,
		mlBaseURL:     fmt.Sprintf("http://%s:8000", host),
		mlAPIKey:      os.Getenv("ML_API_KEY"),
		mlClient:      &http.Client{Timeout: 30 * time.Second},
		trainClient:   &http.Client{Timeout: 7200 * time.Second},
	}
}

// mlSend performs an authenticated HTTP request against the ML service.
// It returns the raw response body, the HTTP status code, and any transport
// error (connection failure etc.). A nil client falls back to h.mlClient.
func (h *AdminHandler) mlSend(method, path string, payload []byte, client *http.Client) ([]byte, int, error) {
	url := h.mlBaseURL + path

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if h.mlAPIKey != "" {
		req.Header.Set("X-ML-API-Key", h.mlAPIKey)
	}
	if client == nil {
		client = h.mlClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// ListUsers returns a paginated list of users from the database.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "50"), c.DefaultQuery("offset", "0"), 50)

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

	dtos := make([]AdminUserDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, toAdminUserDTO(u))
	}

	success(c, gin.H{
		"users":  dtos,
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
		Status   *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		if !isValidRole(*req.Role) {
			fail(c, http.StatusBadRequest, 40001, "无效的角色")
			return
		}
		updates["role"] = *req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Status != nil {
		switch *req.Status {
		case "active":
			updates["is_active"] = true
		case "disabled":
			updates["is_active"] = false
		default:
			fail(c, http.StatusBadRequest, 40001, "无效的状态")
			return
		}
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

// CreateUser handles POST /api/v1/admin/users
// Creates a new user with a specified role (admin only).
func (h *AdminHandler) CreateUser(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, 40001, "用户名、邮箱、密码不能为空")
		return
	}

	role := req.Role
	if role == "" {
		role = "user"
	}
	if !isValidRole(role) {
		fail(c, http.StatusBadRequest, 40001, "无效的角色")
		return
	}

	var existing models.User
	if err := h.DB.Where("username = ? OR email = ?", req.Username, req.Email).First(&existing).Error; err == nil {
		fail(c, http.StatusConflict, 40901, "用户名或邮箱已存在")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, "密码处理失败")
		return
	}

	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         role,
		IsActive:     true,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "创建用户失败")
		return
	}

	success(c, toAdminUserDTO(user))
}

// DeleteUser handles DELETE /api/v1/admin/users/:id
// Deletes a user. The current admin cannot delete their own account.
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	// Compare IDs as unsigned integers to avoid string-equality gaps.
	id := c.Param("id")
	curID, curErr := strconv.ParseUint(middleware.GetUserID(c), 10, 64)
	targetID, targetErr := strconv.ParseUint(id, 10, 64)
	if curErr == nil && targetErr == nil && curID == targetID {
		fail(c, http.StatusBadRequest, 40001, "不能删除当前登录账号")
		return
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		fail(c, http.StatusNotFound, 40401, "用户不存在")
		return
	}

	if err := h.DB.Delete(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "删除用户失败")
		return
	}

	success(c, gin.H{"message": "用户已删除"})
}

// isValidRole reports whether the role is supported by the user model.
func isValidRole(role string) bool {
	return role == "admin" || role == "user"
}

// ListAuditLogs returns a paginated list of audit logs from the database.
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "100"), c.DefaultQuery("offset", "0"), 100)

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
	APIKey           string `json:"apiKey,omitempty"`
	CollectFrequency string `json:"collectFrequency"`
	Enabled          bool   `json:"enabled"`
	LastSyncTime     string `json:"lastSyncTime"`
	Healthy          bool   `json:"healthy"`
}

// GetDataSources handles GET /api/v1/admin/datasources
// Returns a list of real data collectors/data sources with status derived from
// the actual system architecture (TSDB freshness, DB sync, Redis liveness etc).
func (h *AdminHandler) GetDataSources(c *gin.Context) {
	now := time.Now()
	sources := make([]DataSourceInfo, 0, 6)

	appendSource := func(id, name, typ, freq string, enabled, healthy bool, lastSync string) {
		status := "stopped"
		if healthy {
			status = "running"
		}
		sources = append(sources, DataSourceInfo{
			ID:               id,
			Name:             name,
			Type:             typ,
			Status:           status,
			CollectFrequency: freq,
			Enabled:          enabled,
			LastSyncTime:     lastSync,
			Healthy:          healthy,
		})
	}

	// tick freshness drives the real-time/market collectors.
	tickTime, tickOK := h.latestTickTime()
	tickAge := now.Sub(tickTime)
	// 盘外时段数据陈旧属正常（采集节流），仅交易时段内按 5min 新鲜度判定。
	offHours := h.marketService != nil && !h.marketService.IsTradingHours("CN")
	tickFresh := tickOK && tickAge >= 0 && (tickAge < 5*time.Minute || offHours)
	lastTickStr := ""
	if tickOK {
		lastTickStr = tickTime.Format(time.RFC3339)
	}

	// sina: stocks sync state from the main DB.
	sinaHealthy := false
	sinaLastSync := ""
	if h.DB != nil {
		var cnt int64
		if err := h.DB.Model(&models.Stock{}).Count(&cnt).Error; err == nil && cnt > 0 {
			sinaHealthy = true
		}
		var maxUpdated *time.Time
		if err := h.DB.Raw("SELECT MAX(updated_at) AS max_updated FROM stocks").Row().Scan(&maxUpdated); err == nil && maxUpdated != nil {
			sinaLastSync = maxUpdated.Format(time.RFC3339)
		}
	}

	// redis-cache / tsdb liveness.
	redisHealthy := false
	if h.rdb != nil {
		pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		redisHealthy = h.rdb.Ping(pCtx).Err() == nil
		cancel()
	}
	tsdbOK := h.tsdb != nil && h.dbPingOK(h.tsdb)

	appendSource("tencent", "腾讯行情", "rest", "5s", true, tickFresh, lastTickStr)
	appendSource("eastmoney", "东方财富", "rest", "1d", true, tickFresh, lastTickStr)
	appendSource("sina", "新浪财经-股票同步", "rest", "24h", true, sinaHealthy, sinaLastSync)
	appendSource("yahoo", "雅虎财经-海外行情", "rest", "5s", false, false, "")
	appendSource("redis-cache", "Redis 缓存", "rest", "实时", h.rdb != nil, redisHealthy, now.Format(time.RFC3339))
	appendSource("tsdb", "时序数据库", "rest", "实时", tsdbOK, tsdbOK, now.Format(time.RFC3339))

	// Overlay persisted user configuration (enabled / collect_frequency /
	// last_sync_time) onto the real-probed sources. Only fields the user has
	// explicitly saved are overridden; healthy/status stay from the probes.
	h.overlayDataSourceConfig(sources)

	success(c, gin.H{
		"status":  "ok",
		"sources": sources,
	})
}

// dataSourceConfigPrefix is the system_configs key prefix for data sources.
const dataSourceConfigPrefix = "datasource."

// dsConfigKey builds the system_configs key for a data source field.
func dsConfigKey(id, field string) string {
	return dataSourceConfigPrefix + id + "." + field
}

// knownDataSource reports whether id is one of the exposed data sources.
func knownDataSource(id string) bool {
	switch id {
	case "tencent", "eastmoney", "sina", "yahoo", "redis-cache", "tsdb":
		return true
	default:
		return false
	}
}

// setConfig upserts a single system_configs row (create or update by key).
func (h *AdminHandler) setConfig(key, value string) error {
	var cfg models.SystemConfig
	if err := h.DB.Where("key = ?", key).First(&cfg).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return h.DB.Create(&models.SystemConfig{Key: key, Value: value}).Error
	}
	return h.DB.Model(&cfg).Update("value", value).Error
}

// overlayDataSourceConfig mutates sources in place using any persisted
// datasource.* keys. It is a silent no-op when the DB is unavailable or the
// query fails.
func (h *AdminHandler) overlayDataSourceConfig(sources []DataSourceInfo) {
	if h.DB == nil {
		return
	}
	var rows []models.SystemConfig
	if err := h.DB.Where("key LIKE ?", dataSourceConfigPrefix+"%").Find(&rows).Error; err != nil {
		return
	}
	cfg := make(map[string]string, len(rows))
	for _, r := range rows {
		cfg[r.Key] = r.Value
	}
	for i := range sources {
		id := sources[i].ID
		if v, ok := cfg[dsConfigKey(id, "enabled")]; ok {
			if b, err := strconv.ParseBool(v); err == nil {
				sources[i].Enabled = b
			}
		}
		if v, ok := cfg[dsConfigKey(id, "collect_frequency")]; ok && v != "" {
			sources[i].CollectFrequency = v
		}
		if v, ok := cfg[dsConfigKey(id, "last_sync_time")]; ok && v != "" {
			sources[i].LastSyncTime = v
		}
	}
}

// UpdateDataSource handles PUT /api/v1/admin/datasources/:id
// body: {apiKey?, collectFrequency?, enabled?}. Persists any provided field to
// system_configs (keys datasource.<id>.*). Unknown ids are rejected.
func (h *AdminHandler) UpdateDataSource(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}
	id := c.Param("id")
	if !knownDataSource(id) {
		fail(c, http.StatusBadRequest, 40001, "未知的数据源")
		return
	}
	var req struct {
		APIKey           string `json:"apiKey"`
		CollectFrequency string `json:"collectFrequency"`
		Enabled          *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}
	if req.APIKey != "" {
		if err := h.setConfig(dsConfigKey(id, "api_key"), req.APIKey); err != nil {
			log.Printf("[UpdateDataSource] save api_key for %s failed: %v", id, err)
			fail(c, http.StatusInternalServerError, 50001, "保存配置失败")
			return
		}
	}
	if req.CollectFrequency != "" {
		if err := h.setConfig(dsConfigKey(id, "collect_frequency"), req.CollectFrequency); err != nil {
			log.Printf("[UpdateDataSource] save collect_frequency for %s failed: %v", id, err)
			fail(c, http.StatusInternalServerError, 50001, "保存配置失败")
			return
		}
	}
	if req.Enabled != nil {
		if err := h.setConfig(dsConfigKey(id, "enabled"), fmt.Sprintf("%t", *req.Enabled)); err != nil {
			log.Printf("[UpdateDataSource] save enabled for %s failed: %v", id, err)
			fail(c, http.StatusInternalServerError, 50001, "保存配置失败")
			return
		}
	}
	success(c, gin.H{"status": "ok", "message": "配置已保存"})
}

// ToggleDataSource handles PATCH /api/v1/admin/datasources/:id
// body: {enabled bool}. Persists only the enabled flag.
func (h *AdminHandler) ToggleDataSource(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}
	id := c.Param("id")
	if !knownDataSource(id) {
		fail(c, http.StatusBadRequest, 40001, "未知的数据源")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}
	if err := h.setConfig(dsConfigKey(id, "enabled"), fmt.Sprintf("%t", req.Enabled)); err != nil {
		log.Printf("[ToggleDataSource] save enabled for %s failed: %v", id, err)
		fail(c, http.StatusInternalServerError, 50001, "保存配置失败")
		return
	}
	success(c, gin.H{"status": "ok", "message": "配置已保存"})
}

// TriggerDataSourceSync handles POST /api/v1/admin/datasources/:id/sync
// It runs the real sync for the target data source synchronously and returns
// the outcome (plus lastSyncTime persisted to system_configs on success).
func (h *AdminHandler) TriggerDataSourceSync(c *gin.Context) {
	id := c.Param("id")
	if !knownDataSource(id) {
		fail(c, http.StatusBadRequest, 40001, "未知的数据源")
		return
	}
	now := time.Now()

	recordLastSync := func() {
		if h.DB != nil {
			if err := h.setConfig(dsConfigKey(id, "last_sync_time"), now.Format(time.RFC3339)); err != nil {
				log.Printf("[TriggerDataSourceSync] persist last_sync_time for %s failed: %v", id, err)
			}
		}
	}

	switch id {
	case "sina":
		if h.stockService == nil {
			fail(c, http.StatusServiceUnavailable, 50301, "股票同步服务不可用")
			return
		}
		var firstErr error
		for _, mc := range services.ChineseMarketCodes {
			if err := h.stockService.SyncStocksFromMarket(mc); err != nil {
				log.Printf("[TriggerDataSourceSync] sina sync %s failed: %v", mc, err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		// 任一市场失败仅记录，不阻断；同步过程全部执行完毕即视为完成。
		msg := "同步完成"
		if firstErr != nil {
			msg = "同步完成（部分市场失败）"
		}
		recordLastSync()
		success(c, gin.H{"status": "ok", "message": msg, "lastSyncTime": now.Format(time.RFC3339)})
	case "tencent", "eastmoney":
		if h.marketService == nil {
			fail(c, http.StatusServiceUnavailable, 50301, "行情采集服务不可用")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		data, err := h.marketService.CollectMarketData(ctx, "CN")
		if err != nil {
			log.Printf("[TriggerDataSourceSync] %s collect CN failed: %v", id, err)
			fail(c, http.StatusInternalServerError, 50001, "行情采集失败")
			return
		}
		recordLastSync()
		success(c, gin.H{"status": "ok", "message": "同步完成", "count": len(data), "lastSyncTime": now.Format(time.RFC3339)})
	case "redis-cache":
		if h.rdb == nil {
			fail(c, http.StatusServiceUnavailable, 50301, "Redis 缓存不可用")
			return
		}
		pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.rdb.Ping(pCtx).Err(); err != nil {
			fail(c, http.StatusInternalServerError, 50001, "Redis 同步失败")
			return
		}
		recordLastSync()
		success(c, gin.H{"status": "ok", "message": "同步完成", "lastSyncTime": now.Format(time.RFC3339)})
	case "tsdb":
		if h.tsdb == nil {
			fail(c, http.StatusServiceUnavailable, 50301, "时序数据库不可用")
			return
		}
		if !h.dbPingOK(h.tsdb) {
			fail(c, http.StatusInternalServerError, 50001, "时序数据库同步失败")
			return
		}
		recordLastSync()
		success(c, gin.H{"status": "ok", "message": "同步完成", "lastSyncTime": now.Format(time.RFC3339)})
	case "yahoo":
		fail(c, http.StatusBadRequest, 40001, "yahoo 数据源未接入")
		return
	}
}

// ServiceHealthInfo represents a service health check result.
type ServiceHealthInfo struct {
	Name          string `json:"name"`
	Service       string `json:"service"`
	Status        string `json:"status"`
	Latency       int64  `json:"latency"`
	LastHeartbeat string `json:"lastHeartbeat"`
}

// GetServiceHealth handles GET /api/v1/admin/health
// Returns health status of all system services via real probes.
func (h *AdminHandler) GetServiceHealth(c *gin.Context) {
	now := time.Now()
	services := make([]ServiceHealthInfo, 0, 4)

	// gateway: self-check always running.
	services = append(services, ServiceHealthInfo{
		Name:          "API 网关",
		Service:       "gateway",
		Status:        "running",
		Latency:       0,
		LastHeartbeat: now.Format(time.RFC3339),
	})

	// market: healthy when the newest TSDB tick is fresh (< 5 min).
	// 交易时段内数据陈旧视为故障；盘外时段采集节流，进程健康即 running。
	marketSvc := ServiceHealthInfo{
		Name:          "行情服务",
		Service:       "market",
		Status:        "stopped",
		Latency:       0,
		LastHeartbeat: now.Format(time.RFC3339),
	}
	if t, ok := h.latestTickTime(); ok {
		ageMS := now.Sub(t).Milliseconds()
		marketSvc.Latency = ageMS
		marketSvc.LastHeartbeat = t.Format(time.RFC3339)
		if ageMS >= 0 && ageMS < 5*60*1000 {
			marketSvc.Status = "running"
		} else if h.marketService != nil && !h.marketService.IsTradingHours("CN") {
			// 盘外：数据陈旧属正常，标记 running（延迟经 latency 体现）
			marketSvc.Status = "running"
		}
	}
	services = append(services, marketSvc)

	// prediction: HTTP probe to the ML service.
	predSvc := ServiceHealthInfo{
		Name:          "预测服务",
		Service:       "prediction",
		Status:        "stopped",
		Latency:       0,
		LastHeartbeat: now.Format(time.RFC3339),
	}
	if h.mlClient != nil {
		start := time.Now()
		_, statusCode, err := h.mlSend(http.MethodGet, "/api/v1/models/health", nil, nil)
		if err == nil && statusCode == http.StatusOK {
			predSvc.Status = "running"
			predSvc.Latency = time.Since(start).Milliseconds()
		}
	}
	services = append(services, predSvc)

	// engine: healthy when the primary DB responds to a ping.
	engineSvc := ServiceHealthInfo{
		Name:          "交易引擎",
		Service:       "engine",
		Status:        "stopped",
		Latency:       0,
		LastHeartbeat: now.Format(time.RFC3339),
	}
	if h.DB != nil {
		if sqlDB, err := h.DB.DB(); err == nil {
			start := time.Now()
			pCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = sqlDB.PingContext(pCtx)
			cancel()
			if err == nil {
				engineSvc.Status = "running"
				engineSvc.Latency = time.Since(start).Milliseconds()
			}
		}
	}
	services = append(services, engineSvc)

	success(c, gin.H{"services": services})
}

// ErrorLogEntry represents an error log entry.
type ErrorLogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Message   string `json:"message"`
}

// GetErrorLogs handles GET /api/v1/admin/errors
// Returns recent system error logs merged from the main DB (training failures
// + risk events), newest first, capped at 20.
func (h *AdminHandler) GetErrorLogs(c *gin.Context) {
	var logs []ErrorLogEntry
	if h.DB != nil {
		logs = append(logs, h.collectMLFailureLogs()...)
		logs = append(logs, h.collectRiskEventLogs()...)
		sort.SliceStable(logs, func(i, j int) bool {
			return logs[i].Timestamp > logs[j].Timestamp
		})
		if len(logs) > 20 {
			logs = logs[:20]
		}
	}
	if logs == nil {
		logs = []ErrorLogEntry{}
	}
	success(c, gin.H{"status": "ok", "logs": logs})
}

// collectMLFailureLogs maps failed model_training_logs records to error logs.
func (h *AdminHandler) collectMLFailureLogs() []ErrorLogEntry {
	var recs []models.ModelTrainingLog
	if err := h.DB.Where("status = ?", "failed").
		Order("created_at DESC").Limit(20).Find(&recs).Error; err != nil {
		return nil
	}
	out := make([]ErrorLogEntry, 0, len(recs))
	for _, r := range recs {
		ts := r.CreatedAt
		if ts.IsZero() {
			ts = r.StartedAt
		}
		msg := r.ErrorMessage
		if msg == "" {
			msg = "模型训练失败"
		}
		out = append(out, ErrorLogEntry{
			ID:        fmt.Sprint(r.ID),
			Timestamp: ts.Format(time.RFC3339),
			Service:   "ml-service",
			Message:   msg,
		})
	}
	return out
}

// collectRiskEventLogs maps risk_events records to error logs.
func (h *AdminHandler) collectRiskEventLogs() []ErrorLogEntry {
	var recs []models.RiskEvent
	if err := h.DB.Order("created_at DESC").Limit(20).Find(&recs).Error; err != nil {
		return nil
	}
	out := make([]ErrorLogEntry, 0, len(recs))
	for _, r := range recs {
		msg := r.Message
		if r.Details != "" {
			msg = msg + " " + r.Details
		}
		out = append(out, ErrorLogEntry{
			ID:        fmt.Sprint(r.ID),
			Timestamp: r.CreatedAt.Format(time.RFC3339),
			Service:   "sim-trade",
			Message:   msg,
		})
	}
	return out
}

// DataLatencyInfo represents data latency metrics.
type DataLatencyInfo struct {
	RedisHitRate float64 `json:"redisHitRate"`
	UpdateTime   string  `json:"updateTime"`
}

// GetDataLatency handles GET /api/v1/admin/latency
// Returns data pipeline latency metrics computed from real Redis hit stats
// and TSDB tick freshness.
func (h *AdminHandler) GetDataLatency(c *gin.Context) {
	hitRate := 0.0
	if h.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		info, err := h.rdb.Info(ctx, "stats").Result()
		cancel()
		if err == nil {
			hitRate = parseRedisHitRate(info)
		}
	}

	now := time.Now()
	updateTime := now.Format(time.RFC3339)
	latencySec := 0.0
	if t, ok := h.latestTickTime(); ok {
		updateTime = t.Format(time.RFC3339)
		if d := now.Sub(t).Seconds(); d > 0 {
			latencySec = d
		}
	}

	success(c, gin.H{
		"redis_hit_rate":  hitRate,
		"redisHitRate":    hitRate,
		"update_time":     updateTime,
		"updateTime":      updateTime,
		"data_latency_sec": latencySec,
	})
}

// parseRedisHitRate computes keyspace hits/(hits+misses) from `INFO stats`.
func parseRedisHitRate(info string) float64 {
	hits, hitOK := redisStatValue(info, "keyspace_hits")
	misses, missOK := redisStatValue(info, "keyspace_misses")
	if !hitOK || !missOK {
		return 0.0
	}
	total := hits + misses
	if total <= 0 {
		return 0.0
	}
	return hits / total
}

// redisStatValue extracts the numeric value for a `key:value` line in INFO output.
func redisStatValue(info, key string) (float64, bool) {
	prefix := key + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			if f, err := strconv.ParseFloat(strings.TrimPrefix(line, prefix), 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// latestTickTime returns the newest timestamp in stock_prices_tick, if any.
func (h *AdminHandler) latestTickTime() (time.Time, bool) {
	if h.tsdb == nil {
		return time.Time{}, false
	}
	var latest models.StockPriceTick
	if err := h.tsdb.Model(&models.StockPriceTick{}).
		Order("time DESC").First(&latest).Error; err != nil {
		return time.Time{}, false
	}
	if latest.Time.IsZero() {
		return time.Time{}, false
	}
	return latest.Time, true
}

// dbPingOK reports whether a *gorm.DB connection responds to a ping.
func (h *AdminHandler) dbPingOK(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	sqlDB, err := db.DB()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx) == nil
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
// Returns a list of ML models and their real status proxied from the ML service.
func (h *AdminHandler) GetModels(c *gin.Context) {
	data, statusCode, err := h.mlSend(http.MethodGet, "/api/v1/models/status", nil, nil)
	if err != nil {
		success(c, gin.H{
			"status":  "degraded",
			"message": "ML服务不可达",
			"models":  []ModelInfo{},
		})
		return
	}
	if statusCode != http.StatusOK {
		success(c, gin.H{
			"status":  "degraded",
			"message": fmt.Sprintf("ML服务异常（HTTP %d）", statusCode),
			"models":  []ModelInfo{},
		})
		return
	}

	var statuses []struct {
		Period      string  `json:"period"`
		Version     string  `json:"version"`
		Accuracy    float64 `json:"accuracy"`
		LastTrained string  `json:"last_trained"`
		IsHealthy   bool    `json:"is_healthy"`
		ModelType   string  `json:"model_type"`
		Framework   string  `json:"framework"`
	}
	if err := json.Unmarshal(data, &statuses); err != nil {
		success(c, gin.H{
			"status":  "degraded",
			"message": "ML服务响应解析失败",
			"models":  []ModelInfo{},
		})
		return
	}

	models := make([]ModelInfo, 0, len(statuses))
	for _, s := range statuses {
		modelStatus := "ready"
		if !s.IsHealthy {
			modelStatus = "error"
		}
		// Fetch per-model params. A failure here degrades just that model's
		// params to an empty object rather than failing the whole call.
		// ML returns {period, params:{...}} — unwrap to the params map only.
		params := map[string]interface{}{}
		if pdata, pcode, perr := h.mlSend(http.MethodGet, "/api/v1/models/"+s.Period+"/params", nil, nil); perr == nil && pcode == http.StatusOK {
			var p struct {
				Params map[string]interface{} `json:"params"`
			}
			if json.Unmarshal(pdata, &p) == nil && p.Params != nil {
				params = p.Params
			}
		}
		models = append(models, ModelInfo{
			ID:            s.Period,
			Name:          modelDisplayName(s.Period),
			Period:        shortPeriod(s.Period),
			Version:       s.Version,
			Accuracy:      s.Accuracy,
			LastTrainTime: s.LastTrained,
			Status:        modelStatus,
			Params:        params,
		})
	}

	success(c, gin.H{
		"status": "ok",
		"models": models,
	})
}

// ──────────────────────────────────────────────────────────────
// Model training center
// ──────────────────────────────────────────────────────────────

// GetTrainingHistory handles GET /api/v1/admin/training/history
// Returns a paginated list of model training logs, newest first.
func (h *AdminHandler) GetTrainingHistory(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "20"), c.DefaultQuery("offset", "0"), 20)

	var logs []models.ModelTrainingLog
	var total int64

	if err := h.DB.Model(&models.ModelTrainingLog{}).Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询训练历史总数失败")
		return
	}

	if err := h.DB.Limit(limit).Offset(offset).Order("started_at DESC").Find(&logs).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "查询训练历史失败")
		return
	}

	success(c, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// staleTrainingWindow defines how long a training run may stay in "running"
// before it is considered abandoned (e.g. the gateway restarted mid-training).
// Such records are marked failed so they cannot permanently block the same
// period from being trained again.
const staleTrainingWindow = 30 * time.Minute

// RunTraining handles POST /api/v1/admin/training/run
// body: {"period":"short_term"|"medium_term"|"long_term"|"all"}
// Training runs asynchronously: this endpoint records a running
// ModelTrainingLog per target period and returns immediately, while a
// background goroutine proxies the synchronous training call to the ML
// service and updates each log to success/failed when it finishes.
func (h *AdminHandler) RunTraining(c *gin.Context) {
	var req struct {
		Period string `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	periods := trainingTargetPeriods(req.Period)
	if len(periods) == 0 {
		fail(c, http.StatusBadRequest, 40001, "无效的周期")
		return
	}

	now := time.Now()

	if h.DB != nil {
		for _, p := range periods {
			// 训练中已有 running 记录时拒绝重复触发，避免同一模型并发训练。
			// 先清理异常滞留的 running 记录（如服务重启导致的中断），防止永久阻塞。
			h.DB.Model(&models.ModelTrainingLog{}).
				Where("period = ? AND status = ? AND started_at < ?", p, "running", now.Add(-staleTrainingWindow)).
				Updates(map[string]interface{}{
					"status":        "failed",
					"finished_at":   now,
					"error_message": "训练中断（服务重启或超时）",
				})

			var runningCount int64
			if err := h.DB.Model(&models.ModelTrainingLog{}).
				Where("period = ? AND status = ?", p, "running").
				Count(&runningCount).Error; err == nil && runningCount > 0 {
				fail(c, http.StatusBadRequest, 40001, fmt.Sprintf("模型 %s 正在训练中，请稍后再试", p))
				return
			}
		}
	}

	results := make([]gin.H, 0, len(periods))
	logs := make([]*models.ModelTrainingLog, 0, len(periods))
	for _, p := range periods {
		log := models.ModelTrainingLog{
			Period:      p,
			TriggerType: "manual",
			Status:      "running",
			StartedAt:   now,
			CreatedAt:   now,
		}
		if h.DB != nil {
			_ = h.DB.Create(&log).Error
		}
		logs = append(logs, &log)
		results = append(results, gin.H{
			"period":   p,
			"status":   "running",
			"version":  "",
			"accuracy": 0,
			"error":    "",
		})
	}

	// 后台异步训练并回写记录，接口立即返回。
	util.SafeGo(func() {
		for i, p := range periods {
			h.trainOnePeriod(p, logs[i])
		}
	})

	success(c, gin.H{
		"status":  "running",
		"results": results,
		"message": "训练任务已在后台执行",
	})
}

// trainOnePeriod proxies the synchronous training call for one period to the
// ML service and updates the corresponding training log with the outcome.
func (h *AdminHandler) trainOnePeriod(period string, log *models.ModelTrainingLog) {
	data, statusCode, err := h.mlSend(http.MethodPost, "/api/v1/models/train/"+period, nil, h.trainClient)
	if err != nil {
		msg := "ML服务不可达: " + err.Error()
		h.finishTrainingLog(log, "failed", "", 0, msg)
		return
	}

	var res struct {
		Period     string  `json:"period"`
		Status     string  `json:"status"`
		NewVersion string  `json:"new_version"`
		Accuracy   float64 `json:"accuracy"`
	}
	_ = json.Unmarshal(data, &res)

	successLabelled := statusCode == http.StatusOK && (res.Status == "" || res.Status == "success" || res.Status == "ok" || res.Status == "completed")
	if successLabelled {
		h.finishTrainingLog(log, "success", res.NewVersion, res.Accuracy, "")
	} else {
		msg := fmt.Sprintf("ML训练接口返回 %d: %s", statusCode, string(data))
		h.finishTrainingLog(log, "failed", "", 0, msg)
	}
}

// GetTrainingSchedule handles GET /api/v1/admin/training/schedule
// Proxies the ML service's job schedule list.
func (h *AdminHandler) GetTrainingSchedule(c *gin.Context) {
	data, statusCode, err := h.mlSend(http.MethodGet, "/api/v1/models/schedule", nil, nil)
	if err != nil || statusCode != http.StatusOK {
		success(c, gin.H{"status": "degraded", "jobs": []interface{}{}})
		return
	}

	var parsed struct {
		Jobs []interface{} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &parsed); err == nil && parsed.Jobs != nil {
		success(c, gin.H{"status": "ok", "jobs": parsed.Jobs})
		return
	}

	// Fallback: some ML versions return the jobs array bare.
	var jobs []interface{}
	if err := json.Unmarshal(data, &jobs); err == nil && jobs != nil {
		success(c, gin.H{"status": "ok", "jobs": jobs})
		return
	}

	success(c, gin.H{"status": "degraded", "jobs": []interface{}{}})
}

// RollbackModel handles POST /api/v1/admin/training/rollback
// body: {"period": "...", "version": "..."}
// Proxies the model rollback call to the ML service and returns its response.
func (h *AdminHandler) RollbackModel(c *gin.Context) {
	var req struct {
		Period  string `json:"period"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}
	if req.Period == "" || req.Version == "" {
		fail(c, http.StatusBadRequest, 40001, "period 和 version 不能为空")
		return
	}

	payload, _ := json.Marshal(map[string]string{"version": req.Version})
	data, statusCode, err := h.mlSend(http.MethodPost, "/api/v1/models/"+req.Period+"/rollback", payload, nil)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, 50302, "ML服务不可达")
		return
	}

	// Passthrough the ML service's response body.
	var response interface{} = map[string]interface{}{"raw": string(data)}
	_ = json.Unmarshal(data, &response)
	c.JSON(statusCode, response)
}

// ──────────────────────────────────────────────────────────────
// Model params / evaluation / prediction proxies
// ──────────────────────────────────────────────────────────────

// validModelPeriod reports whether period is one of the ML model identifiers.
func validModelPeriod(period string) bool {
	switch period {
	case "short_term", "medium_term", "long_term":
		return true
	default:
		return false
	}
}

// GetModelParams handles GET /api/v1/admin/models/:id/params
// Proxies the model params request to the ML service and returns {period, params}.
func (h *AdminHandler) GetModelParams(c *gin.Context) {
	period := c.Param("id")
	if !validModelPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	data, statusCode, err := h.mlSend(http.MethodGet, "/api/v1/models/"+period+"/params", nil, nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ML服务不可达"})
		return
	}
	if statusCode != http.StatusOK {
		c.JSON(statusCode, gin.H{"error": string(data)})
		return
	}

	// ML returns {period, params:{...}} — unwrap to the params map only.
	var parsed struct {
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		parsed.Params = map[string]interface{}{}
	}
	if parsed.Params == nil {
		parsed.Params = map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "params": parsed.Params})
}

// UpdateModelParams handles PUT /api/v1/admin/models/:id/params
// Proxies the model params update to the ML service and returns
// {period, params, status:"success"} on success.
func (h *AdminHandler) UpdateModelParams(c *gin.Context) {
	period := c.Param("id")
	if !validModelPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	payload, _ := json.Marshal(params)
	data, statusCode, err := h.mlSend(http.MethodPut, "/api/v1/models/"+period+"/params", payload, nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ML服务不可达"})
		return
	}
	if statusCode >= http.StatusBadRequest {
		// Passthrough the ML validation error (e.g. 400).
		msg := string(data)
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			msg = apiErr.Error
		}
		c.JSON(statusCode, gin.H{"error": msg})
		return
	}

	updated := params
	var mlResp struct {
		Params map[string]interface{} `json:"params"`
	}
	if len(data) > 0 && json.Unmarshal(data, &mlResp) == nil && mlResp.Params != nil {
		updated = mlResp.Params
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "params": updated, "status": "success"})
}

// EvaluateModel handles POST /api/v1/admin/models/:id/evaluate
// Proxies a single-period model evaluation to the ML service.
// Evaluation loads the full real dataset and can take minutes, so it runs
// asynchronously: this endpoint returns immediately and a background goroutine
// waits for the ML service result (which writes accuracy back into versions.json).
func (h *AdminHandler) EvaluateModel(c *gin.Context) {
	period := c.Param("id")
	if !validModelPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	util.SafeGo(func() {
		// 评估耗时较长（全量数据加载+前向推理），异步执行；结果由 ML 服务写回 versions.json
		data, statusCode, err := h.mlSend(http.MethodPost, "/api/v1/models/"+period+"/evaluate", nil, h.trainClient)
		if err != nil {
			log.Printf("[EvaluateModel] %s background evaluate failed: %v", period, err)
			return
		}
		if statusCode != http.StatusOK {
			log.Printf("[EvaluateModel] %s background evaluate returned %d: %s", period, statusCode, string(data))
			return
		}
		log.Printf("[EvaluateModel] %s background evaluate done: %s", period, string(data))
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "running",
		"message": "评估已在后台执行，完成后准确率将自动刷新",
	})
}

// PredictModel handles POST /api/v1/admin/models/:id/predict
// Proxies a single-period prediction run to the ML service (predictions/run?period=).
func (h *AdminHandler) PredictModel(c *gin.Context) {
	period := c.Param("id")
	if !validModelPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	data, statusCode, err := h.mlSend(http.MethodPost, "/api/v1/predictions/run?period="+period, nil, nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ML服务不可达"})
		return
	}

	// Passthrough the ML service's response body.
	var response interface{} = map[string]interface{}{"raw": string(data)}
	_ = json.Unmarshal(data, &response)
	c.JSON(statusCode, response)
}

// modelDisplayName returns a human-friendly Chinese name for an ML period.
func modelDisplayName(period string) string {
	switch period {
	case "short_term":
		return "短期LSTM"
	case "medium_term":
		return "中短期集成"
	case "long_term":
		return "长期Transformer"
	default:
		return period
	}
}

// shortPeriod strips the "_term" suffix so the DB/frontend convention
// short/medium/long is used: "short_term" -> "short".
func shortPeriod(period string) string {
	return strings.TrimSuffix(period, "_term")
}

// trainingTargetPeriods expands a requested period into a list of target
// ML periods ("all" means all three). Empty result means an invalid period.
func trainingTargetPeriods(period string) []string {
	switch period {
	case "short_term":
		return []string{"short_term"}
	case "medium_term":
		return []string{"medium_term"}
	case "long_term":
		return []string{"long_term"}
	case "all":
		return []string{"short_term", "medium_term", "long_term"}
	default:
		return nil
	}
}

// finishTrainingLog marks a training log with its final outcome. It is a
// no-op when the DB is unavailable or the log was never persisted.
func (h *AdminHandler) finishTrainingLog(log *models.ModelTrainingLog, status, version string, accuracy float64, errMsg string) {
	if h.DB == nil || log == nil || log.ID == 0 {
		return
	}
	finishedAt := time.Now()
	durationSec := int(finishedAt.Sub(log.StartedAt).Seconds())
	updates := map[string]interface{}{
		"status":        status,
		"finished_at":   &finishedAt,
		"duration_sec":  durationSec,
		"error_message": errMsg,
	}
	if version != "" {
		updates["version"] = version
	}
	if status == "success" {
		updates["accuracy"] = accuracy
	}
	_ = h.DB.Model(log).Updates(updates).Error
}
