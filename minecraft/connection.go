package minecraft

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"net"
	"time"
)

func (s *Server) listen(port string) error {
	listener, err := net.Listen("tcp", ":" + port)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go s.newPlayer(conn)
	}
}

func(s *Server) newPlayer(p net.Conn) {
	current := player{connection: p}
	handshakeState, err := current.readHandshake()
	if err != nil {
		_ = current.connection.Close()
		panic(err)
	}

	// https://wiki.vg/Server_List_Ping
	if *handshakeState == 1 {
		defer current.connection.Close()

		// Request packet
		current.getNextPacket()

		// Response packet
		response := new(Packet)
		response.ID = handshakePacketID

		// JSON response
		_, _ = String("{\"version\": {\"name\": \"1.16.5\",\"protocol\": 754},\"players\": {\"max\": 10,\"online\": 5},\"description\": {\"text\": \"Minecraft Light Server Go\"}}").WriteTo(response)

		if err := response.Pack(current.connection); err != nil {
			panic(err)
		}

		// Ping
		ping := current.getNextPacket()
		var pingPayload Long
		if _, err := pingPayload.ReadFrom(ping); err != nil {
			panic(err)
		}

		// Pong
		pong := new(Packet)
		pong.ID = 0x01
		_, _ = pingPayload.WriteTo(pong)
		if err := pong.Pack(current.connection); err != nil {
			panic(err)
		}

		return
	} else { // state 2
		// Login start
		loginStart := current.getNextPacket()
		var username String
		_, _ = username.ReadFrom(loginStart)

		current.id = UUID(uuid.New())

		// Login success
		if loginStart.ID == handshakePacketID {
			success := new(Packet)
			success.ID = 0x02
			_, _ = current.id.WriteTo(success)
			_, _ = username.WriteTo(success)
			if err := success.Pack(current.connection); err != nil {
				panic(err)
			}

			// Save current player in players sync map
			s.players.Store(current.id, current)
		} else {
			_ = current.connection.Close()
			panic(errors.New("invalid login packet id"))
		}
	}

	// Set player parameters and allow him to join the game

	// Keep Alive goroutine
	go func() {
		for {
			keepAlive := new(Packet)
			keepAlive.ID = keepAlivePacketID
			_, _ = Long(rand.Int63()).WriteTo(keepAlive)

			// Connection error, remove client from players
			if err := keepAlive.Pack(current.connection); err != nil {
				s.players.Delete(current.id)
				return
			}

			// send keep alive every 20 seconds
			time.Sleep(time.Second * 20)
		}
	}()
}