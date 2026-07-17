package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/api"
	"github.com/ldm2060/axonhub/internal/server/backup"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/dependencies"
	"github.com/ldm2060/axonhub/internal/server/gc"
	"github.com/ldm2060/axonhub/internal/server/gql"
	"github.com/ldm2060/axonhub/internal/server/gql/openapi"
	"github.com/ldm2060/axonhub/internal/server/middleware"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
	"github.com/ldm2060/axonhub/internal/server/video_storage"
	"github.com/ldm2060/axonhub/internal/tracing"
)

func New(config Config) *Server {
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(middleware.Recovery())

	return &Server{
		Config: config,
		Engine: engine,
	}
}

type Server struct {
	*gin.Engine

	Config      Config
	server      *http.Server
	pprofServer *http.Server
	addr        string
}

func (srv *Server) newHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      srv.Engine,
		ReadTimeout:  srv.Config.ReadTimeout,
		WriteTimeout: 0,
	}
}

func (srv *Server) Run() error {
	log.Info(context.Background(), "run server",
		log.String("name", srv.Config.Name),
		log.String("host", srv.Config.Host),
		log.Int("port", srv.Config.Port),
	)
	addr := fmt.Sprintf("%s:%d", srv.Config.Host, srv.Config.Port)
	srv.server = srv.newHTTPServer(addr)
	srv.addr = addr

	if srv.Config.Pprof.Enabled {
		pprofAddr := fmt.Sprintf("%s:%d", srv.Config.Pprof.Host, srv.Config.Pprof.Port)
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		srv.pprofServer = &http.Server{
			Addr:              pprofAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		log.Info(context.Background(), "starting pprof server",
			log.String("addr", pprofAddr))

		go func() {
			if err := srv.pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error(context.Background(), "pprof server error", log.Cause(err))
			}
		}()
	}

	err := srv.server.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}

	return nil
}

func (srv *Server) Shutdown(ctx context.Context) error {
	if srv.pprofServer != nil {
		if err := srv.pprofServer.Shutdown(ctx); err != nil {
			log.Error(ctx, "pprof server shutdown error", log.Cause(err))
		}
	}

	return srv.server.Shutdown(ctx)
}

func Run(opts ...fx.Option) {
	constructors := []any{
		openapi.NewGraphqlHandlers,
		gql.NewGraphqlHandlers,
		gc.NewWorker,
		New,
		NewIPAccessControlRuntime,
	}

	app := fx.New(
		append([]fx.Option{
			fx.WithLogger(func() fxevent.Logger {
				return &fxevent.ConsoleLogger{W: os.Stderr}
			}),
			fx.Provide(constructors...),
			dependencies.Module,
			scheduler.Module,
			biz.Module,
			orchestrator.Module,
			backup.Module,
			video_storage.Module,
			api.Module,
			fx.Provide(func(cfg Config) api.TimeoutConfig {
				return api.TimeoutConfig{
					LLMRequestTimeout:    cfg.LLMRequestTimeout,
					LLMStreamIdleTimeout: cfg.LLMStreamIdleTimeout,
				}
			}),
			fx.Provide(fx.Annotate(func(cfg Config) string { return cfg.PublicURL }, fx.ResultTags(`name:"public_url"`))),
			fx.Invoke(func(cfg log.Config) {
				log.SetGlobalConfig(cfg)
				tracing.SetupLogger(log.GetGlobalLogger())
				slog.SetDefault(log.GetGlobalLogger().AsSlog())
			}),
			fx.Invoke(func(usageLogSvc *biz.UsageLogService) {
				usageLogSvc.OnUsageLogCreated = gql.InvalidateAllTimeTokenStatsCache
			}),
			fx.Invoke(func(cfg Config) {
				if cfg.Dashboard.AllTimeTokenStatsSoftTTL > 0 && cfg.Dashboard.AllTimeTokenStatsHardTTL > 0 {
					gql.SetTokenStatsCacheTTL(cfg.Dashboard.AllTimeTokenStatsSoftTTL, cfg.Dashboard.AllTimeTokenStatsHardTTL)
				}
			}),
			fx.Invoke(func(lc fx.Lifecycle, worker *gc.Worker, s *scheduler.Scheduler) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						return worker.RegisterScheduledTasks(ctx, s)
					},
				})
			}),
			fx.Invoke(SetupRoutes),
		}, opts...)...,
	)
	app.Run()
}
