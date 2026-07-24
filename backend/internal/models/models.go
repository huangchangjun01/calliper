package models

import (
	"time"

	"gorm.io/datatypes"
)

// Market represents a stock exchange market.
type Market struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Code          string         `gorm:"type:varchar(20);uniqueIndex;not null" json:"code"`
	Name          string         `gorm:"type:varchar(100);not null" json:"name"`
	NameCN        string         `gorm:"type:varchar(100)" json:"name_cn"`
	Country       string         `gorm:"type:varchar(50);not null" json:"country"`
	Currency      string         `gorm:"type:varchar(10);not null" json:"currency"`
	Timezone      string         `gorm:"type:varchar(50);not null" json:"timezone"`
	TradingHours  string         `gorm:"type:text" json:"trading_hours"`
	Status        string         `gorm:"type:varchar(20);default:active;not null" json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Stocks        []Stock        `gorm:"foreignKey:MarketID" json:"stocks,omitempty"`
}

// Stock represents a tradable stock.
type Stock struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Symbol       string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Name         string         `gorm:"type:varchar(200);not null" json:"name"`
	NameCN       string         `gorm:"type:varchar(200)" json:"name_cn"`
	MarketID     uint           `gorm:"not null;index" json:"market_id"`
	Exchange     string         `gorm:"type:varchar(50)" json:"exchange"`
	Industry     string         `gorm:"type:varchar(100)" json:"industry"`
	Sector       string         `gorm:"type:varchar(100)" json:"sector"`
	MarketCap    float64        `gorm:"type:decimal(20,2)" json:"market_cap"`
	Currency     string         `gorm:"type:varchar(10);default:CNY" json:"currency"`
	LotSize      int            `gorm:"default:100" json:"lot_size"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Market       Market         `gorm:"foreignKey:MarketID" json:"market,omitempty"`
}

// User represents a system user.
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Role         string         `gorm:"type:varchar(20);default:user;not null" json:"role"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Watchlist represents a user's watchlist entry.
type Watchlist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_user_stock" json:"user_id"`
	StockID   uint      `gorm:"not null;uniqueIndex:idx_user_stock" json:"stock_id"`
	AddedAt   time.Time `gorm:"autoCreateTime" json:"added_at"`
	User      User      `gorm:"foreignKey:UserID" json:"-"`
	Stock     Stock     `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// Order represents a trading order.
type Order struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	StockID       uint           `gorm:"not null;index" json:"stock_id"`
	OrderType     string         `gorm:"type:varchar(10);not null" json:"order_type"`
	OrderKind     string         `gorm:"type:varchar(10);not null" json:"order_kind"`
	Price         float64        `gorm:"type:decimal(18,4)" json:"price"`
	Quantity      int            `gorm:"not null" json:"quantity"`
	FilledQuantity int           `gorm:"default:0" json:"filled_quantity"`
	Status        string         `gorm:"type:varchar(20);default:pending;not null" json:"status"`
	IsReal        bool           `gorm:"default:false" json:"is_real"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	User          User           `gorm:"foreignKey:UserID" json:"-"`
	Stock         Stock          `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// Position represents a user's holding position.
type Position struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;uniqueIndex:idx_user_stock_position" json:"user_id"`
	StockID        uint      `gorm:"not null;uniqueIndex:idx_user_stock_position" json:"stock_id"`
	Quantity       int       `gorm:"not null;default:0" json:"quantity"`
	AvgCost        float64   `gorm:"type:decimal(18,4)" json:"avg_cost"`
	CurrentValue   float64   `gorm:"type:decimal(20,2)" json:"current_value"`
	UnrealizedPnL  float64   `gorm:"type:decimal(20,2)" json:"unrealized_pnl"`
	RealizedPnL    float64   `gorm:"type:decimal(20,2)" json:"realized_pnl"`
	IsReal         bool      `gorm:"default:false" json:"is_real"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	User           User      `gorm:"foreignKey:UserID" json:"-"`
	Stock          Stock     `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// SimulatedTrade represents a simulated/paper trade record.
type SimulatedTrade struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StockID      uint      `gorm:"not null;index" json:"stock_id"`
	TradeType    string    `gorm:"type:varchar(10);not null" json:"trade_type"`
	Price        float64   `gorm:"type:decimal(18,4);not null" json:"price"`
	Quantity     int       `gorm:"not null" json:"quantity"`
	Confidence   float64   `gorm:"type:decimal(5,2)" json:"confidence"`
	PredictionID *uint     `json:"prediction_id"`
	Reason       string    `gorm:"type:text" json:"reason"`
	ExecutedAt   time.Time `gorm:"not null" json:"executed_at"`
	Stock        Stock     `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// Prediction represents an AI/ML prediction for a stock.
type Prediction struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	StockID     uint           `gorm:"not null;index" json:"stock_id"`
	Period      string         `gorm:"type:varchar(20);not null" json:"period"`
	Direction   string         `gorm:"type:varchar(10);not null" json:"direction"`
	Confidence  float64        `gorm:"type:decimal(5,2)" json:"confidence"`
	TargetPrice float64        `gorm:"type:decimal(18,4)" json:"target_price"`
	Factors     datatypes.JSON `gorm:"type:jsonb" json:"factors"`
	ModelVersion string        `gorm:"type:varchar(50)" json:"model_version"`
	PredictedAt time.Time      `gorm:"not null;index" json:"predicted_at"`
	ValidUntil  time.Time      `json:"valid_until"`
	Success     *bool          `gorm:"type:boolean" json:"success,omitempty"`
	Stock       Stock          `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// PredictionAccuracy tracks the accuracy of predictions.
type PredictionAccuracy struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	StockID            uint      `gorm:"not null" json:"stock_id"`
	PredictionID       uint      `gorm:"not null;index" json:"prediction_id"`
	PredictedDirection string    `gorm:"type:varchar(10);not null" json:"predicted_direction"`
	ActualDirection    string    `gorm:"type:varchar(10);not null" json:"actual_direction"`
	IsCorrect          bool      `gorm:"not null" json:"is_correct"`
	Period             string    `gorm:"type:varchar(20);not null" json:"period"`
	EvaluatedAt        time.Time `gorm:"not null" json:"evaluated_at"`
	Stock              Stock     `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// SystemConfig stores system-wide configuration key-value pairs.
type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"key"`
	Value       string    `gorm:"type:text;not null" json:"value"`
	Description string    `gorm:"type:text" json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AuditLog records system audit events.
type AuditLog struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     *uint          `json:"user_id"`
	Action     string         `gorm:"type:varchar(50);not null;index" json:"action"`
	Resource   string         `gorm:"type:varchar(50);not null" json:"resource"`
	ResourceID uint           `json:"resource_id"`
	Details    datatypes.JSON `gorm:"type:jsonb" json:"details"`
	IPAddress  string         `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt  time.Time      `gorm:"autoCreateTime;index" json:"created_at"`
}

// SimAccount represents a simulated trading account.
type SimAccount struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TotalAssets   float64   `gorm:"type:decimal(20,2);not null" json:"total_assets"`
	AvailableCash float64   `gorm:"type:decimal(20,2);not null" json:"available_cash"`
	FrozenCash    float64   `gorm:"type:decimal(20,2);default:0" json:"frozen_cash"`
	MarketValue   float64   `gorm:"type:decimal(20,2);default:0" json:"market_value"`
	TotalPnL      float64   `gorm:"type:decimal(20,2);default:0" json:"total_pnl"`
	TodayPnL      float64   `gorm:"type:decimal(20,2);default:0" json:"today_pnl"`
	TodayReturn   float64   `gorm:"type:decimal(10,4);default:0" json:"today_return"`
	StartDate     string    `gorm:"type:varchar(10);not null" json:"start_date"`
	IsRunning     bool      `gorm:"default:false" json:"is_running"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RiskEvent records a risk control event triggered during simulated trading.
type RiskEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventType string    `gorm:"type:varchar(50);not null;index" json:"event_type"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	Details   string    `gorm:"type:text" json:"details"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// ──────────────────────────────────────────────────────────────
// TimescaleDB hypertable models (time-series price data)
// ──────────────────────────────────────────────────────────────

// StockPriceMinute represents 1-minute aggregate stock price data.
type StockPriceMinute struct {
	Time    time.Time `gorm:"not null" json:"time"`
	StockID uint      `gorm:"not null;uniqueIndex:idx_minute_stock_time" json:"stock_id"`
	Open    float64   `gorm:"type:decimal(18,4)" json:"open"`
	High    float64   `gorm:"type:decimal(18,4)" json:"high"`
	Low     float64   `gorm:"type:decimal(18,4)" json:"low"`
	Close   float64   `gorm:"type:decimal(18,4)" json:"close"`
	Volume  int64     `json:"volume"`
	Amount  float64   `gorm:"type:decimal(20,2)" json:"amount"`
}

// StockPriceDaily represents daily aggregate stock price data.
type StockPriceDaily struct {
	Time            time.Time `gorm:"not null" json:"time"`
	StockID         uint      `gorm:"not null;uniqueIndex:idx_daily_stock_time" json:"stock_id"`
	Open            float64   `gorm:"type:decimal(18,4)" json:"open"`
	High            float64   `gorm:"type:decimal(18,4)" json:"high"`
	Low             float64   `gorm:"type:decimal(18,4)" json:"low"`
	Close           float64   `gorm:"type:decimal(18,4)" json:"close"`
	Volume          int64     `json:"volume"`
	Amount          float64   `gorm:"type:decimal(20,2)" json:"amount"`
	TurnoverRate    float64   `gorm:"type:decimal(10,4)" json:"turnover_rate"`
	PERatio         float64   `gorm:"type:decimal(10,2)" json:"pe_ratio"`
	PBRatio         float64   `gorm:"type:decimal(10,2)" json:"pb_ratio"`
	TotalMarketCap  float64   `gorm:"type:decimal(20,2)" json:"total_market_cap"`
	FloatMarketCap  float64   `gorm:"type:decimal(20,2)" json:"float_market_cap"`
}

// StockPriceTick represents tick-level stock trade data.
type StockPriceTick struct {
	Time      time.Time `gorm:"not null" json:"time"`
	StockID   uint      `gorm:"not null;index:idx_tick_stock_time" json:"stock_id"`
	Price     float64   `gorm:"type:decimal(18,4);not null" json:"price"`
	Volume    int64     `json:"volume"`
	Direction string    `gorm:"type:varchar(10)" json:"direction"`
}

// ──────────────────────────────────────────────────────────────
// TableName overrides
// ──────────────────────────────────────────────────────────────

func (Market) TableName() string             { return "markets" }
func (Stock) TableName() string              { return "stocks" }
func (User) TableName() string               { return "users" }
func (Watchlist) TableName() string          { return "watchlists" }
func (Order) TableName() string              { return "orders" }
func (Position) TableName() string           { return "positions" }
func (SimulatedTrade) TableName() string     { return "simulated_trades" }
func (Prediction) TableName() string         { return "predictions" }
func (PredictionAccuracy) TableName() string  { return "prediction_accuracies" }
func (SystemConfig) TableName() string       { return "system_configs" }
func (AuditLog) TableName() string           { return "audit_logs" }
func (SimAccount) TableName() string          { return "sim_accounts" }
func (RiskEvent) TableName() string           { return "risk_events" }
func (StockPriceMinute) TableName() string   { return "stock_prices_1min" }
func (StockPriceDaily) TableName() string    { return "stock_prices_daily" }
func (StockPriceTick) TableName() string     { return "stock_prices_tick" }