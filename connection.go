package MinecraftLightServer

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
	current := Player{connection: p}
	handshakeState, err := current.readHandshake()
	if err != nil {
		current.panicAndCloseConnection(err)
	}

	// https://wiki.vg/Server_List_Ping
	if *handshakeState == 1 {
		defer current.connection.Close()

		// Request packet
		current.getNextPacket()

		// Response packet
		response := NewPacket(handshakePacketID, nil)

		// JSON response
		_, _ = String("{\"version\": {\"name\": \"1.16.5\",\"protocol\": 754},\"players\": {\"max\": 10,\"online\": 5},\"description\": {\"text\": \"Minecraft Light Server Go\"}}").WriteTo(response)
		if err := response.Pack(current.connection); err != nil {
			panic(err)
		}

		// Ping
		ping := current.getNextPacket()
		var pingPayload Long
		if _, err := pingPayload.ReadFrom(ping); err != nil {
			current.panicAndCloseConnection(err)
		}

		// Pong
		pong := NewPacket(handshakePong, []PacketFieldWrite{pingPayload})
		if err := pong.Pack(current.connection); err != nil {
			current.panicAndCloseConnection(err)
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
			success := NewPacket(handshakeLoginSuccess, []PacketFieldWrite{current.id, username})
			if err := success.Pack(current.connection); err != nil {
				panic(err)
			}

			// Save current Player in players sync map
			s.players.Store(current.id, &current)
		} else {
			current.panicAndCloseConnection(errors.New("invalid login packet id"))
		}
	}

	// Set Player parameters and allow him to join the game

	// Keep Alive goroutine
	go s.keepAliveUser(&current)
}

func (s *Server) keepAliveUser(current *Player) {
	for {
		// Keep Alive packet with random int
		random := Long(rand.Int63())
		keepAlive := NewPacket(keepAlivePacketID, []PacketFieldWrite{random})

		// if there is a connection error remove client from players map
		if err := keepAlive.Pack(current.connection); err != nil {
			s.players.Delete(current.id)
			return
		}

		// send keep alive every 20 seconds
		time.Sleep(time.Second * 20)
	}
}