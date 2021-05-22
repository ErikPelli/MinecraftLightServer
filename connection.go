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
	listener, err := net.Listen("tcp", ":"+port)
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

func (s *Server) newPlayer(p net.Conn) {
	current := Player{connection: p, x: 0, y: 5, z: 0, yawAbs: 0, pitchAbs: 0}
	var username String
	handshakeState, err := current.readHandshake()
	if err != nil {
		current.closeGoroutineAndConnection(err)
	}

	// https://wiki.vg/Server_List_Ping
	if *handshakeState == 1 {
		defer current.connection.Close()

		// Request packet
		_, _ = current.getNextPacket()

		// Response packet
		response := NewPacket(handshakePacketID)

		// JSON response
		_, _ = String("{\"version\": {\"name\": \"1.16.5\",\"protocol\": 754},\"players\": {\"max\": 10,\"online\": 5},\"description\": {\"text\": \"Minecraft Light Server Go\"}}").WriteTo(response)
		if err := response.Pack(current.connection); err != nil {
			panic(err)
		}

		// Ping
		ping, err := current.getNextPacket()
		if err != nil {
			current.closeGoroutineAndConnection(err)
		}

		var pingPayload Long
		if _, err := pingPayload.ReadFrom(ping); err != nil {
			current.closeGoroutineAndConnection(err)
		}

		// Pong
		pong := NewPacket(handshakePong, pingPayload)
		if err := pong.Pack(current.connection); err != nil {
			current.closeGoroutineAndConnection(err)
		}

		return
	} else { // state 2
		// Login start
		loginStart, err := current.getNextPacket()
		if err != nil {
			current.closeGoroutineAndConnection(err)
		}

		_, _ = username.ReadFrom(loginStart)

		current.id = UUID(uuid.New())

		// Login success
		if loginStart.ID == handshakePacketID {
			success := NewPacket(handshakeLoginSuccess, current.id, username)
			if err := success.Pack(current.connection); err != nil {
				panic(err)
			}

			// Save current Player in players sync map
			s.players.Store(username, &current)
			s.incrementCounter()
		} else {
			current.closeGoroutineAndConnection(errors.New("invalid login packet id"))
		}
	}

	// Set Player parameters
	if err := current.joinGame(); err != nil {
		current.closeGoroutineAndConnection(err)
	}
	if err := current.writePlayerPosition(
		current.x, current.y, current.z,
		current.yawAbs, current.pitchAbs,
		Byte(0x00), VarInt(current.getIntfromUUID())); err != nil {
		current.closeGoroutineAndConnection(err)
	}
	if err := current.writeServerDifficulty(); err != nil {
		current.closeGoroutineAndConnection(err)
	}

	if err := current.writeChunk(Int(0), Int(0)); err != nil {
		current.closeGoroutineAndConnection(err)
	}
	if err := current.writeChunk(Int(0), Int(1)); err != nil {
		current.closeGoroutineAndConnection(err)
	}
	if err := current.writeChunk(Int(1), Int(0)); err != nil {
		current.closeGoroutineAndConnection(err)
	}
	if err := current.writeChunk(Int(1), Int(1)); err != nil {
		current.closeGoroutineAndConnection(err)
	}

	// Send information to other clients
	s.broadcastPlayerInfo()
	s.broadcastChatMessage(string(username)+" joined the server", "Server")
	s.broadcastSpawnPlayer()

	// User packets handler goroutine

	// Keep Alive goroutine
	go s.keepAliveUser(&current)
}

func (s *Server) keepAliveUser(current *Player) {
	for {
		// Keep Alive packet with random int
		random := Long(rand.Int63())
		keepAlive := NewPacket(keepAlivePacketID, random)

		// if there is a connection error remove client from players map
		if err := keepAlive.Pack(current.connection); err != nil {
			s.players.Delete(current.id)
			s.decrementCounter()
			return
		}

		// send keep alive every 20 seconds
		time.Sleep(time.Second * 20)
	}
}

func (s *Server) broadcastPlayerInfo() {
	s.players.Range(func(key interface{}, value interface{}) bool {
		// Send packet to current host
		broadcast := NewPacket(broadcastPlayerInfoPacketID,
			VarInt(0),         // add player
			VarInt(s.counter), // number of players
		)

		s.players.Range(func(key interface{}, value interface{}) bool {
			// Add every player to packet
			_, _ = value.(*Player).id.WriteTo(broadcast) // player uuid
			_, _ = key.(String).WriteTo(broadcast)       // username
			_, _ = VarInt(0).WriteTo(broadcast)          // no properties
			_, _ = VarInt(0).WriteTo(broadcast)          // gamemode 0 (survival)
			_, _ = VarInt(123).WriteTo(broadcast)        // hardcoded ping
			_, _ = Boolean(false).WriteTo(broadcast)     // has display name

			return true
		})

		// Send players packet
		_ = broadcast.Pack(value.(*Player).connection)
		return true
	})
}

func (s *Server) broadcastChatMessage(msg, username string) {
	s.players.Range(func(key interface{}, value interface{}) bool {
		player := value.(*Player)
		if err := player.writeChat(msg, username); err != nil {
			player.closeGoroutineAndConnection(err)
		}
		return true
	})
}

func (s *Server) broadcastSpawnPlayer() {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		currentPlayer := playerInterface.(*Player)
		s.players.Range(func(key interface{}, p interface{}) bool {
			// Get all other players
			otherPlayer := p.(*Player)
			if currentPlayer.id != otherPlayer.id {
				_ = currentPlayer.writeSpawnPlayer(
					VarInt(otherPlayer.getIntfromUUID()),
					otherPlayer.id,
					otherPlayer.x,
					otherPlayer.y,
					otherPlayer.z,
					otherPlayer.yaw,
					otherPlayer.pitch,
				)
				_ = currentPlayer.writeEntityLook(
					VarInt(otherPlayer.getIntfromUUID()),
					otherPlayer.yaw,
				)
			}
			return true
		})
		return true
	})
}