package services

import (
	"encoding/json"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// AuditService handles audit logging for trading operations.
type AuditService struct {
	db *gorm.DB
}

// NewAuditService creates a new AuditService.
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// LogTrade records a trade-related audit log entry.
func (s *AuditService) LogTrade(userID uint, action string, resource string, resourceID string, details map[string]interface{}) {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	log := models.AuditLog{
		UserID:  &userID,
		Action:  action,
		Resource: resource,
		IPAddress: "127.0.0.1",
		Details: datatypes.JSON(detailsJSON),
	}

	if resourceID != "" {
		// Store resourceID as string representation in details if needed
		_ = resourceID
	}

	if s.db != nil {
		_ = s.db.Create(&log)
	}
}

// LogAccess records an access audit log entry.
func (s *AuditService) LogAccess(userID uint, resource string, action string) {
	log := models.AuditLog{
		UserID:    &userID,
		Action:    action,
		Resource:  resource,
		IPAddress: "127.0.0.1",
		Details:   datatypes.JSON([]byte("{}")),
	}

	if s.db != nil {
		_ = s.db.Create(&log)
	}
}