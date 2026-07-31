package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/config"
	"github.com/quant-trading/backend/internal/database"
	"github.com/quant-trading/backend/internal/handlers"
	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/services"
	ws "github.com/quant-trading/backend/internal/websocket"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Redis client
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	// Verify Redis connection
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v (rate limiting disabled)", err)
		rdb = nil
	}

	// Initialize database
	dbCfg := database.DefaultConfig()
	var db *gorm.DB
	if err := database.Initialize(dbCfg); err != nil {
		log.Printf("Warning: Database connection failed: %v (stock search disabled)", err)
	} else {
		db = database.GetDB()
	}

	// Initialize stock service
	var stockService *services.StockService
	if db != nil {
		stockService = services.NewStockService(db, rdb)
	}

	// Initialize market data service
	var tsdb *gorm.DB
	if db != nil {
		tsdb = database.GetTSDB()
	}
	// Only configure Kafka if brokers are actually set; otherwise the
	// market data service will use a no-op producer to avoid panics.
	var kafkaBrokers []string
	if strings.TrimSpace(cfg.KafkaBrokers) != "" {
		kafkaBrokers = strings.Split(cfg.KafkaBrokers, ",")
	}
	marketService := services.NewMarketDataService(services.MarketDataServiceConfig{
		TSDB:         tsdb,
		Redis:        rdb,
		KafkaBrokers: kafkaBrokers,
		MLServiceURL: "http://ml-service:8000",
	})

	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware([]string{"*"}))

	// Rate limiting (if Redis is available)
	if rdb != nil {
		router.Use(middleware.RateLimitMiddleware(rdb, 100, time.Minute))
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "quant-trading-api-gateway",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Background context for long-running services
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Initialize Quote Subscription Manager
	subManager := services.NewQuoteSubscriptionManager()

	// Initialize Quote Push Service
	quotePushService := services.NewQuotePushService(hub, rdb, tsdb)
	quotePushService.Start(bgCtx)

	// Set data callback: when MarketDataService collects data, push to WebSocket
	marketService.SetDataCallback(func(data []services.MarketData) {
		quotePushService.PushBatch(data)
	})

	// Start market data collection loop (periodic polling from Sina/Yahoo Finance)
	marketService.StartCollection(bgCtx)

	// Initialize watchlist service
	var watchlistService *services.WatchlistService
	if db != nil {
		watchlistService = services.NewWatchlistService(db, rdb, quotePushService)
	}

	// WebSocket handler
	wsHandler := handlers.NewWsHandler(hub, cfg.JWTSecret, subManager, quotePushService)
	router.GET("/ws", wsHandler.HandleWebSocket)

	// Initialize trading components
	mockBroker := services.NewMockBroker(db)
	tradeService := services.NewTradeService(db, mockBroker, rdb)
	tradeHandler := handlers.NewTradeHandler(tradeService)

	// Initialize prediction service
	mlServiceURL := fmt.Sprintf("http://%s:8000", getEnvDefault("ML_SERVICE_HOST", "ml-service"))
	predictionService := services.NewPredictionService(mlServiceURL)
	predictionHandler := handlers.NewPredictionHandler(predictionService)

	// Initialize simulated trading components
	var simTradeService *services.SimTradeService
	var accountService *services.AccountService
	var positionManager *services.PositionManager
	var simTradeHandler *handlers.SimTradeHandler
	if db != nil {
		accountService = services.NewAccountService(db, rdb)
		positionManager = services.NewPositionManager(db)
		simTradeService = services.NewSimTradeService(db, predictionService, rdb, positionManager, accountService, marketService)
		simTradeHandler = handlers.NewSimTradeHandler(simTradeService, accountService, positionManager)

		// Initialize sim account on first run
		if _, err := accountService.GetAccount(); err != nil {
			log.Printf("Warning: Failed to initialize sim account: %v", err)
		}

		// Auto-start sim trading scheduler
		simTradeService.StartScheduler(bgCtx)
	}

	// Initialize evaluation service
	evaluationService := services.NewEvaluationService(db, tsdb)
	evaluationScheduler := services.NewEvaluationScheduler(evaluationService)
	evaluationHandler := handlers.NewEvaluationHandler(evaluationService, evaluationScheduler)

	// Wire evaluation service to prediction handler for accuracy endpoints
	predictionHandler.SetEvaluationService(evaluationService)

	// Start evaluation scheduler
	evaluationScheduler.Start(bgCtx)

	// Initialize admin handler
	adminHandler := handlers.NewAdminHandler(db)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg.JWTSecret, cfg.JWTExpiration, db)
	stockHandler := handlers.NewStockHandler(stockService)
	marketHandler := handlers.NewMarketHandler(marketService)
	var watchlistHandler *handlers.WatchlistHandler
	if watchlistService != nil {
		watchlistHandler = handlers.NewWatchlistHandler(watchlistService)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
			auth.POST("/refresh", middleware.AuthMiddleware(cfg.JWTSecret), authHandler.RefreshToken)
		}

		// Protected routes (requires authentication)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		{
			// Stocks
			stocks := protected.Group("/stocks")
			{
				stocks.GET("/search", stockHandler.SearchStocks)
				stocks.GET("/market/:code", stockHandler.GetStocksByMarket)
				stocks.GET("/health", stockHandler.HealthCheck)
				stocks.POST("/sync/:market", stockHandler.SyncMarket)
				stocks.GET("/:symbol", stockHandler.GetStockBySymbol)

				// Watchlist (requires watchlist service)
				if watchlistHandler != nil {
					stocks.GET("/watchlist", watchlistHandler.GetWatchlist)
					stocks.POST("/watchlist/:symbol", watchlistHandler.AddToWatchlist)
					stocks.DELETE("/watchlist/:symbol", watchlistHandler.RemoveFromWatchlist)
				}
			}

			// Market data
			market := protected.Group("/market")
			{
				market.POST("/realtime/batch", marketHandler.GetRealtimeBatch)
				market.GET("/realtime/:symbol", marketHandler.GetRealtime)
				market.GET("/kline/:symbol", marketHandler.GetKline)
				market.GET("/depth/:symbol", marketHandler.GetDepth)
				market.GET("/backfill/progress", marketHandler.GetBackfillProgress)
				market.POST("/backfill", marketHandler.TriggerBackfill)
				market.GET("/indices", marketHandler.GetIndices)
				market.GET("/statistics", marketHandler.GetMarketStatistics)
				market.GET("/fundamentals/:symbol", marketHandler.GetFundamentals)
			}

			// Predictions
			predictions := protected.Group("/predictions")
			{
				predictions.GET("/summaries", predictionHandler.GetSummaries)
				predictions.GET("/details", predictionHandler.GetDetails)
				predictions.GET("/accuracy", predictionHandler.GetAccuracyTrend)
				predictions.GET("/stock-accuracy", predictionHandler.GetStockAccuracy)
				predictions.GET("/failures", predictionHandler.GetFailures)
				predictions.GET("/accuracy/:symbol", predictionHandler.GetPredictionAccuracy)
				predictions.GET("/:symbol/history", predictionHandler.GetPredictionHistory)
				predictions.GET("/:symbol", predictionHandler.GetPrediction)
				predictions.POST("/batch", predictionHandler.BatchPredict)
			}

			// Evaluation
			evaluation := protected.Group("/evaluation")
			{
				evaluation.GET("/accuracy/:symbol/stats", evaluationHandler.GetAccuracyStats)
				evaluation.GET("/accuracy/:symbol", evaluationHandler.GetAccuracy)
				evaluation.GET("/ranking", evaluationHandler.GetRanking)
				evaluation.GET("/metrics/:symbol", evaluationHandler.GetMetrics)
				evaluation.GET("/failure/:symbol", evaluationHandler.GetFailureAnalysis)
			}

			// Trading
			trading := protected.Group("/trading")
			{
				trading.POST("/order", tradeHandler.PlaceOrder)
				trading.DELETE("/order/:id", tradeHandler.CancelOrder)
				trading.GET("/orders", tradeHandler.GetOrders)
				trading.GET("/order/:id", tradeHandler.GetOrderByID)
				trading.GET("/positions", tradeHandler.GetPositions)
				trading.GET("/account", tradeHandler.GetAccount)

				// Simulated trading
				if simTradeHandler != nil {
					sim := trading.Group("/sim")
					{
						sim.GET("/status", simTradeHandler.GetStatus)
						sim.POST("/start", simTradeHandler.StartSimTrading)
						sim.POST("/stop", simTradeHandler.StopSimTrading)
						sim.GET("/decisions", simTradeHandler.GetDecisions)
						sim.GET("/account", simTradeHandler.GetAccount)
						sim.GET("/positions", simTradeHandler.GetPositions)
						sim.GET("/trades", simTradeHandler.GetTrades)
					}
				}
			}
		}

		// Admin routes (requires admin role)
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		admin.Use(middleware.AdminMiddleware())
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/users/:id", adminHandler.GetUser)
			admin.PUT("/users/:id", adminHandler.UpdateUser)
			admin.GET("/audit-log", adminHandler.ListAuditLogs)
			admin.GET("/system/status", adminHandler.GetSystemStatus)
			admin.POST("/predictions/run", predictionHandler.TriggerPrediction)
			admin.GET("/models/status", predictionHandler.GetModelStatus)
			admin.POST("/evaluation/run", evaluationHandler.RunEvaluation)
			admin.GET("/datasources", adminHandler.GetDataSources)
			admin.GET("/health", adminHandler.GetServiceHealth)
			admin.GET("/errors", adminHandler.GetErrorLogs)
			admin.GET("/latency", adminHandler.GetDataLatency)
			admin.GET("/models", adminHandler.GetModels)
		}
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.ServerPort),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("API Gateway starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down API Gateway...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close Redis connection
	if rdb != nil {
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}

	// Close database connections
	database.Close()

	log.Println("API Gateway stopped")
}

// getEnvDefault returns the environment variable value or a default.
func getEnvDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}