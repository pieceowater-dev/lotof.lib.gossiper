package fiber

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

type Server struct {
	Port      string
	App       *fiber.App
	InitRoute func(app *fiber.App)
}

func New(port string, app *fiber.App, initRoute func(app *fiber.App)) *Server {
	if initRoute != nil {
		initRoute(app)
	}
	return &Server{
		Port:      port,
		App:       app,
		InitRoute: initRoute,
	}
}

func (r *Server) Start() error {
	slog.Info("REST server running", "port", r.Port)
	return r.App.Listen(":" + r.Port)
}

func (r *Server) Stop() error {
	slog.Info("REST server stopping")
	return r.App.Shutdown()
}
