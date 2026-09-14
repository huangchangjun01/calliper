package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/models"
)

// AccountHandler handles personal account API requests (authenticated users).
type AccountHandler struct {
	DB *gorm.DB
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(db *gorm.DB) *AccountHandler {
	return &AccountHandler{DB: db}
}

// GetMe returns the current user's profile. GET /api/v1/account/me
func (h *AccountHandler) GetMe(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	success(c, toAdminUserDTO(user))
}

// UpdateMe updates the current user's profile. PUT /api/v1/account/me
func (h *AccountHandler) UpdateMe(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req struct {
		Email *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	if req.Email == nil {
		fail(c, http.StatusBadRequest, 40001, "没有需要更新的字段")
		return
	}

	email := strings.TrimSpace(*req.Email)
	if email == "" {
		fail(c, http.StatusBadRequest, 40001, "邮箱不能为空")
		return
	}
	if len(email) > 100 {
		fail(c, http.StatusBadRequest, 40001, "邮箱长度不能超过100")
		return
	}

	// 检查邮箱是否被其他用户占用
	var existing models.User
	if err := h.DB.Where("email = ? AND id <> ?", email, user.ID).First(&existing).Error; err == nil {
		fail(c, http.StatusConflict, 40901, "邮箱已被其他账号使用")
		return
	}

	if err := h.DB.Model(&user).Update("email", email).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "更新资料失败")
		return
	}

	user.Email = email
	success(c, toAdminUserDTO(user))
}

// ChangePassword updates the current user's password. PUT /api/v1/account/me/password
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	if h.DB == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "数据库不可用")
		return
	}

	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "无效的请求参数")
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		fail(c, http.StatusBadRequest, 40001, "旧密码和新密码不能为空")
		return
	}
	if len(req.NewPassword) < 6 {
		fail(c, http.StatusBadRequest, 40001, "新密码长度不能少于6位")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		fail(c, http.StatusBadRequest, 40001, "旧密码不正确")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, "密码处理失败")
		return
	}

	if err := h.DB.Model(&user).Update("password_hash", string(hashedPassword)).Error; err != nil {
		fail(c, http.StatusInternalServerError, 50001, "修改密码失败")
		return
	}

	success(c, gin.H{"message": "密码修改成功"})
}

// currentUser loads the authenticated user from the JWT user_id.
func (h *AccountHandler) currentUser(c *gin.Context) (models.User, bool) {
	idStr := middleware.GetUserID(c)
	if idStr == "" {
		fail(c, http.StatusUnauthorized, 40101, "未认证")
		return models.User{}, false
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, "无效的用户身份")
		return models.User{}, false
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		fail(c, http.StatusNotFound, 40401, "用户不存在")
		return models.User{}, false
	}

	return user, true
}
