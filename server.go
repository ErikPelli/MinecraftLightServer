package MinecraftLightServer

import (
	"sync"
)

const serverPort = "25565" // default listen port

type Server struct {
	port string
	players sync.Map
	counter int // number of users online
	counterMut sync.Mutex
}

func NewServer() *Server {
	s := new(Server)
	s.port = serverPort
	return s
}

func(s *Server) SetPort(port string) {
	s.port = port
}

func(s *Server) Start() error {
	err := s.listen(serverPort)
	if err != nil {
		return err
	}

	return nil
}

func(s *Server) incrementCounter() {
	s.counterMut.Lock()
	s.counter++
	s.counterMut.Unlock()
}

func(s *Server) decrementCounter() {
	s.counterMut.Lock()
	if s.counter > 0 {
		s.counter--
	}
	s.counterMut.Unlock()
}