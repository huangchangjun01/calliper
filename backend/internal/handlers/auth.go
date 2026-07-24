package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quant-trading/backend/internal/middleware"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	JWTSecret       string
	JWTExpiration   int
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(secret string, expiration int) *AuthHandler {
	return &AuthHandler{
		JWTSecret:     secret,
		JWTExpiration: expiration,
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

	// TODO: Replace with actual user authentication against database
	userID := "placeholder-user-id"
	role := "user"

	token, err := middleware.GenerateToken(userID, role, h.JWTSecret, h.JWTExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:  token,
		UserID: userID,
		Role:   role,
	})
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

	// TODO: Replace with actual user registration in database
	userID := "placeholder-new-user-id"

	c.JSON(http.StatusCreated, RegisterResponse{
		UserID:  userID,
		Message: "user registered successfully",
	})
}

// RefreshTokenResponse represents a token refresh response.
type RefreshTokenResponse struct {
	Token string `json:"token"`
}

// RefreshToken handles JWT token refresh.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	token, err := middleware.GenerateToken(userID, role, h.JWTSecret, h.JWTExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, RefreshTokenResponse{
		Token: token,
	})
}