package servers

import (
	"log"
	"sync"
)

type Server interface {
	Start() error
	Stop() error
}

type ServerManager struct {
	servers []Server
}

func NewServerManager() *ServerManager {
	return &ServerManager{}
}

func (sm *ServerManager) AddServer(server Server) {
	sm.servers = append(sm.servers, server)
}

func (sm *ServerManager) StartAll() {

	var wg sync.WaitGroup
	for _, server := range sm.servers {
		wg.Add(1)
		go func(s Server) {
			defer wg.Done()
			if err := s.Start(); err != nil {
				log.Printf("Error starting server: %v", err)
			}
		}(server)
	}
	wg.Wait()

}

func (sm *ServerManager) StopAll() {
	for _, server := range sm.servers {
		if err := server.Stop(); err != nil {
			log.Printf("Error stopping server: %v", err)
		}
	}
}
