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