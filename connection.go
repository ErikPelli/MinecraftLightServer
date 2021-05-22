package MinecraftLightServer

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
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
	current := Player{
		connection: p,
		x: 0,
		y: 5,
		z: 0,
		yawAbs: 0, pitchAbs: 0,
		pitch: 0, yaw: 0,
		onGround: true,
	}

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

		_, _ = current.username.ReadFrom(loginStart)

		current.id = UUID(uuid.New())

		// Login success
		if loginStart.ID == handshakePacketID {
			success := NewPacket(handshakeLoginSuccess, current.id, current.username)
			if err := success.Pack(current.connection); err != nil {
				panic(err)
			}

			// Save current Player in players sync map
			s.players.Store(current.id, &current)
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
	s.broadcastChatMessage(string(current.username)+" joined the server", "Server")
	s.broadcastSpawnPlayer()

	// User packets handler goroutine
	go current.handlePacket(s)

	// Keep Alive goroutine
	go s.keepAliveUser(&current)
}

func (p *Player) handlePacket(s *Server) {
	for {
		packet, err := p.getNextPacket()
		if err != nil {
			p.closeGoroutineAndConnection(err)
		}

		switch packet.ID {
		case readTeleportConfirmPacketID:
			// Do nothing

		case readChatPacketID:
			var message String
			if _, err := message.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			s.broadcastChatMessage(string(message), string(p.username))

		case readKeepAlivePacketID:
			// Do nothing

		case readPositionPacketID:
			if _, err := p.x.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.y.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.z.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.getIntfromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

		case readPositionAndLookPacketID:
			if _, err := p.x.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.y.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.z.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.yawAbs.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.pitchAbs.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}

			// Calculate yaw and pitch
			p.yaw = p.yawAbs.toAngle()
			p.pitch = p.pitchAbs.toAngle()

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.getIntfromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

		case readRotationPacketID:
			if _, err := p.yawAbs.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.pitchAbs.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}

			// Calculate yaw and pitch
			p.yaw = p.yawAbs.toAngle()
			p.pitch = p.pitchAbs.toAngle()

			// Send to other players
			s.broadcastPlayerRotation(VarInt(p.getIntfromUUID()), p.yaw, p.pitch, p.onGround)

		case readEntityActionPacketID:
			_, _ = new(VarInt).ReadFrom(packet) // discard entity id

			var actionID VarInt
			if _, err := actionID.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			s.broadcastEntityAction(VarInt(p.getIntfromUUID()), actionID)

		case readAnimationPacketID:
			var animationID VarInt
			if _, err := animationID.ReadFrom(packet); err != nil {
				p.closeGoroutineAndConnection(err)
			}
			s.broadcastEntityAnimation(VarInt(p.getIntfromUUID()), animationID)

		default:
			// log unknown packet
			fmt.Printf("[%d] Unknown packet: 0x%02X\n", p.getIntfromUUID(), packet.ID)
		}
	}
}