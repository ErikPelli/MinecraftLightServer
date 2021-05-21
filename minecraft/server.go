package minecraft

import (
	"sync"
)

const serverPort = "25565" // default listen port

type Server struct {
	port string
	players sync.Map
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