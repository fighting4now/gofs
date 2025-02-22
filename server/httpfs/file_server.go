package httpfs

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/no-src/gofs"
	"github.com/no-src/gofs/auth"
	"github.com/no-src/gofs/core"
	"github.com/no-src/gofs/driver/minio"
	"github.com/no-src/gofs/driver/sftp"
	"github.com/no-src/gofs/report"
	"github.com/no-src/gofs/server"
	"github.com/no-src/gofs/server/handler"
	"github.com/no-src/gofs/server/middleware"
	"github.com/no-src/log"
)

// StartFileServer start a file server by gin
func StartFileServer(opt server.Option) error {
	logger := opt.Logger
	initEnvGinMode()
	gin.DefaultWriter = logger

	engine := gin.New()
	engine.NoRoute(middleware.NoRoute)

	initCompress(engine, opt.EnableFileServerCompress)
	initDefaultMiddleware(engine, logger)
	if err := initHTMLTemplate(engine); err != nil {
		opt.Init.DoneWithError(err)
		return err
	}
	if err := initSession(engine, opt.SessionMode, opt.SessionConnection); err != nil {
		opt.Init.DoneWithError(err)
		return err
	}
	if err := initRoute(engine, opt, logger); err != nil {
		opt.Init.DoneWithError(err)
		return err
	}

	log.Info("file server [%s] starting...", opt.Addr)
	server.InitServerInfo(opt.Addr, opt.EnableTLS)
	opt.Init.Done()

	if opt.EnableTLS {
		return engine.RunTLS(opt.Addr, opt.TLSCertFile, opt.TLSKeyFile)
	}
	log.Warn("file server is not a security connection, you need the https replaced maybe!")
	return engine.Run(opt.Addr)
}

// initEnvGinMode change default mode is release
func initEnvGinMode() {
	mode := os.Getenv(gin.EnvGinMode)
	if mode == "" {
		mode = gin.ReleaseMode
	}
	gin.SetMode(mode)
}

func initSession(engine *gin.Engine, sessionMode int, sessionConnection string) error {
	store, err := server.NewSessionStore(sessionMode, sessionConnection)
	if err != nil {
		return err
	}
	engine.Use(sessions.Sessions(server.SessionName, store))
	return nil
}

func initDefaultMiddleware(engine *gin.Engine, logger io.Writer) {
	engine.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: defaultLogFormatter,
		Output:    logger,
		SkipPaths: []string{"/favicon.ico"},
	}), gin.Recovery(), middleware.ApiStat())
}

func initHTMLTemplate(engine *gin.Engine) error {
	tmpl, err := template.ParseFS(gofs.Templates, server.ResourceTemplatePath)
	if err != nil {
		log.Error(err, "parse template fs error")
		return err
	}
	engine.SetHTMLTemplate(tmpl)
	return nil
}

func initCompress(engine *gin.Engine, enableCompress bool) {
	if enableCompress {
		// enable gzip compression
		engine.Use(gzip.Gzip(gzip.DefaultCompression))
	}
}

func initRoute(engine *gin.Engine, opt server.Option, logger log.Logger) error {
	enableFileApi := false
	source := opt.Source
	dest := opt.Dest

	loginGroup := engine.Group(server.LoginGroupRoute)
	loginGroup.GET(server.LoginIndexRoute, func(context *gin.Context) {
		context.HTML(http.StatusOK, "login.html", nil)
	})
	loginGroup.POST(server.LoginSignInRoute, handler.NewLoginHandlerFunc(opt.Users, logger))

	rootGroup := engine.Group(server.RootGroupRoute)
	wGroup := engine.Group(server.WriteGroupRoute)
	manageGroup := engine.Group(server.ManageGroupRoute)

	initRouteAuth(opt, logger, rootGroup, wGroup, manageGroup)

	rootGroup.GET(server.DefaultRoute, handler.NewDefaultHandlerFunc(logger))

	initManageRoute(opt, logger, manageGroup)

	if source.IsDisk() || source.Is(core.RemoteDisk) {
		rootGroup.StaticFS(server.SourceRoutePrefix, http.Dir(source.Path()))
		enableFileApi = true

		if opt.EnablePushServer {
			wGroup.POST(server.PushRoute, handler.NewPushHandlerFunc(logger, source, opt.EnableLogicallyDelete))
		}
	}

	if dest.IsDisk() {
		rootGroup.StaticFS(server.DestRoutePrefix, http.Dir(dest.Path()))
		enableFileApi = true
	} else if dest.Is(core.SFTP) {
		if len(opt.Users) == 0 {
			return errors.New("a user is required for sftp server")
		}
		user := opt.Users[0]
		sftpDir, err := sftp.NewDir(dest.RemotePath(), dest.Addr(), user.UserName(), user.Password(), opt.SSHKey, opt.Retry)
		if err != nil {
			return err
		}
		rootGroup.StaticFS(server.DestRoutePrefix, sftpDir)
		enableFileApi = true
	} else if dest.Is(core.MinIO) {
		if len(opt.Users) == 0 {
			return errors.New("a user is required for MinIO server")
		}
		user := opt.Users[0]
		sftpDir, err := minio.NewDir(dest.RemotePath(), dest.Addr(), dest.Secure(), user.UserName(), user.Password(), opt.Retry)
		if err != nil {
			return err
		}
		rootGroup.StaticFS(server.DestRoutePrefix, sftpDir)
		enableFileApi = true
	}

	if enableFileApi {
		rootGroup.GET(server.QueryRoute, handler.NewFileApiHandlerFunc(logger, http.Dir(source.Path()), opt.ChunkSize, opt.CheckpointCount))
	}

	return nil
}

func initRouteAuth(opt server.Option, logger log.Logger, rootGroup, wGroup, manageGroup *gin.RouterGroup) {
	if len(opt.Users) > 0 {
		rootGroup.Use(middleware.NewAuthHandlerFunc(logger, auth.ReadPerm))
		wGroup.Use(middleware.NewAuthHandlerFunc(logger, auth.WritePerm))
		manageGroup.Use(middleware.NewAuthHandlerFunc(logger, auth.ExecutePerm))
	} else {
		server.PrintAnonymousAccessWarning()
	}
}

func initManageRoute(opt server.Option, logger log.Logger, manageGroup *gin.RouterGroup) {
	if opt.EnableManage {
		if opt.ManagePrivate {
			manageGroup.Use(middleware.NewPrivateAccessHandlerFunc(logger))
		}
		pprof.RouteRegister(manageGroup, server.PProfRoutePrefix)
		manageGroup.GET(server.ManageConfigRoute, handler.NewManageHandlerFunc(logger))
		if opt.EnableReport {
			manageGroup.GET(server.ManageReportRoute, handler.NewReportHandlerFunc(logger))
			report.GlobalReporter.Enable(true)
		}
	}
}

var defaultLogFormatter = func(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		// Truncate in a golang < 1.8 safe way
		param.Latency = param.Latency - param.Latency%time.Second
	}
	return fmt.Sprintf("[%v] [GIN] |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006-01-02 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)
}
