package MinecraftLightServer

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
)

func (s *Server) listen(portNumber chan string, errChannel chan error) {
	var listener net.Listener
	isListening := true

	go func() {
		for newPort := range portNumber {
			if listener != nil {
				_ = listener.Close()
			}

			var err error
			listener, err = net.Listen("tcp", ":"+newPort)
			errChannel <- err
		}

		close(errChannel)
		isListening = false
	}()

	for isListening {
		// Check if listener has been initialized
		if listener != nil {
			conn, err := listener.Accept()

			// Handle request if there aren't errors
			if err == nil {
				go s.newPlayer(conn)
			}
		}
	}

	_ = listener.Close()
}

func (s *Server) newPlayer(p net.Conn) {
	current := Player{
		connection: p,
		id:			UUID(uuid.New()),
		isDeleted:  false,
		x:          0,
		y:          5,
		z:          0,
		yawAbs:     0,
		pitchAbs:   0,
		pitch:      0,
		yaw:        0,
		onGround:   true,
	}

	// Get client handshake packet
	handshake, err := current.getNextPacket()
	if err != nil {
		s.removePlayerAndExit(&current, err)
	} else if handshake.ID != handshakePacketID {
		s.removePlayerAndExit(&current, errors.New("wrong handshake packet id"))
	}

	// Parse packet and save next state field
	handshakeNextState, err := current.readHandshake(handshake)
	if err != nil {
		s.removePlayerAndExit(&current, err)
	}

	// https://wiki.vg/Server_List_Ping
	if *handshakeNextState == 1 {
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
			s.removePlayerAndExit(&current, err)
		}

		var pingPayload Long
		if _, err := pingPayload.ReadFrom(ping); err != nil {
			s.removePlayerAndExit(&current, err)
		}

		// Pong
		pong := NewPacket(handshakePong, pingPayload)
		if err := pong.Pack(current.connection); err != nil {
			s.removePlayerAndExit(&current, err)
		}

		return
	} else { // state 2
		// Login start
		loginStart, err := current.getNextPacket()
		if err != nil {
			s.removePlayerAndExit(&current, err)
		}

		_, _ = current.username.ReadFrom(loginStart)

		// Login success
		if loginStart.ID == handshakePacketID {
			success := NewPacket(handshakeLoginSuccess, current.id, current.username)
			if err := success.Pack(current.connection); err != nil {
				panic(err)
			}

			// Save current Player in players sync map
			s.addPlayer(&current)
		} else {
			s.removePlayerAndExit(&current, errors.New("invalid login packet id"))
		}
	}

	// Set Player parameters
	if err := current.writeJoinGame(); err != nil {
		s.removePlayerAndExit(&current, err)
	}
	if err := current.writePlayerPosition(
		current.x, current.y, current.z,
		current.yawAbs, current.pitchAbs,
		Byte(0x00), VarInt(current.getIntFromUUID())); err != nil {
		s.removePlayerAndExit(&current, err)
	}
	if err := current.writeServerDifficulty(); err != nil {
		s.removePlayerAndExit(&current, err)
	}

	// send 4 chunks to client
	chunks := [][]Int{{-1, 0}, {0, 0}, {-1, -1}, {0, -1}}
	for _, position := range chunks {
		if err := current.writeChunk(position[0], position[1]); err != nil {
			s.removePlayerAndExit(&current, err)
		}
	}

	// Send information to other clients
	s.broadcastPlayerInfo()
	s.broadcastChatMessage(string(current.username)+" joined the server", "Server")
	s.broadcastSpawnPlayer()

	// User packets handler goroutine
	go s.handlePacket(&current)

	// Keep Alive goroutine
	go s.keepAliveUser(&current)
}

func (s *Server) handlePacket(p *Player) {
	for {
		packet, err := p.getNextPacket()
		if err != nil {
			s.removePlayerAndExit(p, err)
		}

		switch packet.ID {
		case readTeleportConfirmPacketID:
			// Do nothing

		case readChatPacketID:
			var message String
			if _, err := message.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			s.broadcastChatMessage(string(message), string(p.username))

		case readKeepAlivePacketID:
			// Do nothing

		case readPositionPacketID:
			// Old position
			oldX := p.x
			oldZ := p.z

			if _, err := p.x.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.y.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.z.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}

			// Update player chunk view if chunk has changed
			if p.z != oldZ || convertCoordinatesToChunk(p.x) != convertCoordinatesToChunk(oldX) {
				if err := p.updateViewPosition(); err != nil {
					s.removePlayerAndExit(p, err)
				}
			}

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.getIntFromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

		case readPositionAndLookPacketID:
			// Old position
			oldX := p.x
			oldZ := p.z

			if _, err := p.x.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.y.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.z.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.yawAbs.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.pitchAbs.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}

			// Calculate yaw and pitch
			p.yaw = p.yawAbs.toAngle()
			p.pitch = p.pitchAbs.toAngle()

			// Update player chunk view if chunk has changed
			if p.z != oldZ || convertCoordinatesToChunk(p.x) != convertCoordinatesToChunk(oldX) {
				if err := p.updateViewPosition(); err != nil {
					s.removePlayerAndExit(p, err)
				}
			}

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.getIntFromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

		case readRotationPacketID:
			if _, err := p.yawAbs.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.pitchAbs.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			if _, err := p.onGround.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}

			// Calculate yaw and pitch
			p.yaw = p.yawAbs.toAngle()
			p.pitch = p.pitchAbs.toAngle()

			// Send to other players
			s.broadcastPlayerRotation(VarInt(p.getIntFromUUID()), p.yaw, p.pitch, p.onGround)

		case readEntityActionPacketID:
			_, _ = new(VarInt).ReadFrom(packet) // discard entity id

			var actionID VarInt
			if _, err := actionID.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			s.broadcastEntityAction(VarInt(p.getIntFromUUID()), actionID)

		case readAnimationPacketID:
			var animationID VarInt
			if _, err := animationID.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			s.broadcastEntityAnimation(VarInt(p.getIntFromUUID()), animationID)

		default:
			// log unknown packet
			fmt.Printf("[%s] Unknown packet: 0x%02X\n", p.username, packet.ID)
		}
	}
}
