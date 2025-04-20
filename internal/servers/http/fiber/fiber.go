package fiber

import (
	"github.com/gofiber/fiber/v2"
	"log"
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
	log.Print("\033[32m")
	log.Printf("REST server running on port %s", r.Port)
	log.Print("\033[0m")
	return r.App.Listen(":" + r.Port)
}

func (r *Server) Stop() error {
	log.Println("REST server stopping")
	return r.App.Shutdown()
}
