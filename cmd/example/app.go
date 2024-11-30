package main

import (
	"github.com/gin-gonic/gin"
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"google.golang.org/grpc"
)

func main() {
	// Создаём менеджер серверов
	serverManager := gossiper.NewServerManager()

	// Инициализация gRPC сервера
	grpcInitRoute := func(server *grpc.Server) {
		// Пример: добавить маршруты
	}
	serverManager.AddServer(gossiper.NewGRPCServ("50051", grpc.NewServer(), grpcInitRoute))

	// Инициализация REST сервера
	restInitRoute := func(router *gin.Engine) {
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}
	serverManager.AddServer(gossiper.NewRESTServ("8080", gin.Default(), restInitRoute))

	// Запуск всех серверов
	serverManager.StartAll()
	defer serverManager.StopAll()
}
