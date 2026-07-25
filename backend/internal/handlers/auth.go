package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/models"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	JWTSecret     string
	JWTExpiration int
	DB            *gorm.DB
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(secret string, expiration int, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		JWTSecret:     secret,
		JWTExpiration: expiration,
		DB:            db,
	}
}

// LoginRequest represents a login request body.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token    string `json:"token"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
}

// Login handles user login and returns a JWT token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Authenticate against database
	if h.DB != nil {
		var user models.User
		if err := h.DB.Where("username = ? AND is_active = ?", req.Username, true).First(&user).Error; err == nil {
			if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err == nil {
				userID := strconv.FormatUint(uint64(user.ID), 10)
				token, err := middleware.GenerateToken(userID, user.Role, h.JWTSecret, h.JWTExpiration)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
					return
				}
				c.JSON(http.StatusOK, LoginResponse{
					Token:  token,
					UserID: userID,
					Role:   user.Role,
				})
				return
			}
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication service unavailable"})
}

// RegisterRequest represents a registration request body.
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required"`
}

// RegisterResponse represents a registration response.
type RegisterResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// Register handles new user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if h.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	// Check if user already exists
	var existing models.User
	if err := h.DB.Where("username = ? OR email = ?", req.Username, req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process password"})
		return
	}

	// Create user
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         "user",
		IsActive:     true,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create user: %v", err)})
		return
	}

	userID := strconv.FormatUint(uint64(user.ID), 10)
	c.JSON(http.StatusCreated, RegisterResponse{
		UserID:  userID,
		Message: "user registered successfully",
	})
}

// RefreshToken issues a new JWT token for the authenticated user.
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	token, err := middleware.GenerateToken(userID, role, h.JWTSecret, h.JWTExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": userID,
		"role":    role,
	})
}