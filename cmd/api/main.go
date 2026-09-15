package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/yourusername/hoas-booking/config"
	appauth "github.com/yourusername/hoas-booking/internal/application/auth"
	appbooking "github.com/yourusername/hoas-booking/internal/application/booking"
	infraauth "github.com/yourusername/hoas-booking/internal/infrastructure/auth"
	"github.com/yourusername/hoas-booking/internal/infrastructure/email"
	"github.com/yourusername/hoas-booking/internal/infrastructure/http/handler"
	"github.com/yourusername/hoas-booking/internal/infrastructure/http/middleware"
	"github.com/yourusername/hoas-booking/internal/infrastructure/jobs"
	"github.com/yourusername/hoas-booking/internal/infrastructure/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}

	var logger *zap.Logger
	if cfg.IsProd() { logger, _ = zap.NewProduction() } else { logger, _ = zap.NewDevelopment() }
	defer logger.Sync()

	db, err := postgres.NewDB(cfg.Database)
	if err != nil { logger.Fatal("connect to database", zap.Error(err)) }
	defer db.Close()

	m, err := migrate.New("file://migrations", cfg.Database.MigrationURL())
	if err != nil { logger.Fatal("create migrator", zap.Error(err)) }
	if err := m.Up(); err != nil && err != migrate.ErrNoChange { logger.Fatal("run migrations", zap.Error(err)) }
	logger.Info("database migrations up to date")

	// Repositories
	bookingRepo  := postgres.NewBookingRepository(db)
	userRepo     := postgres.NewUserRepository(db)
	machineRepo  := postgres.NewMachineRepository(db)
	quotaRepo    := postgres.NewQuotaRepository(db)
	waitlistRepo := postgres.NewWaitlistRepository(db)

	// Infrastructure services
	jwtService := infraauth.NewJWTService(cfg.JWT)
	_ = infraauth.NewMockBankIDProvider(cfg.BankID)
	notifier := email.NewMockNotifier(logger)

	// Use cases
	registerUC      := appauth.NewRegisterUseCase(userRepo)
	loginUC         := appauth.NewLoginUseCase(userRepo)
	createLaundryUC := appbooking.NewCreateLaundryBookingUseCase(bookingRepo, machineRepo, quotaRepo, userRepo, waitlistRepo, notifier)
	cancelLaundryUC := appbooking.NewCancelLaundryBookingUseCase(bookingRepo, waitlistRepo, notifier)
	createSaunaUC   := appbooking.NewCreateSaunaBookingUseCase(bookingRepo, quotaRepo, notifier)

	// HTTP server
	e := echo.New()
	e.HideBanner = true
	e.Validator = handler.NewRequestValidator()

	e.Use(echomiddleware.RequestID())
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{cfg.App.FrontendURL},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentType},
		AllowCredentials: true,
	}))

	e.GET("/health", func(c echo.Context) error {
		if err := db.PingContext(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	authMW := middleware.JWTAuth(jwtService)

	authHandler := handler.NewAuthHandler(registerUC, loginUC, jwtService, userRepo)
	authHandler.RegisterRoutes(e)

	bookingHandler := handler.NewBookingHandler(createLaundryUC, cancelLaundryUC, createSaunaUC, bookingRepo)
	bookingHandler.RegisterRoutes(e, authMW)

	scheduler := jobs.NewScheduler(logger, bookingRepo, quotaRepo, waitlistRepo)
	scheduler.Start()
	defer scheduler.Stop()

	go func() {
		addr := ":" + cfg.App.Port
		logger.Info("server starting", zap.String("addr", addr), zap.String("env", cfg.App.Env))
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = e.Shutdown(ctx)
}
