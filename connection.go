package MinecraftLightServer

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
)

// listen starts listening for minecraft clients and
// use portNumber channel to change listening port.
func (s *Server) listen(portNumber <-chan string, errChannel chan<- error) {
	var listener net.Listener
	isListening := true

	go func() {
		for newPort := range portNumber {
			// Close old listener
			if listener != nil {
				_ = listener.Close()
			}

			// Listen on new port and send error to channel
			var err error
			listener, err = net.Listen("tcp", ":"+newPort)
			errChannel <- err
		}

		// Stop listening when port channel has been closed
		close(errChannel)
		isListening = false
	}()

	// Handle requests while server is listening
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

	// Close listener
	_ = listener.Close()
}

// newPlayer initializes a new client connected, using its connection.
func (s *Server) newPlayer(conn net.Conn) {
	current := Player{
		connection: conn,
		id:         UUID(uuid.New()),
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

	// Parse handshake packet and save next state field
	handshakeNextState, err := current.readHandshake(handshake)
	if err != nil {
		s.removePlayerAndExit(&current, err)
	}

	if *handshakeNextState == 1 {
		// Close the connection at the end of ping-pong
		defer current.connection.Close()

		// Discard request packet
		_, _ = current.getNextPacket()

		// Response packet (JSON)
		if err := NewPacket(handshakePacketID,
			String("{\"version\": {\"name\": \"1.16.5\",\"protocol\": 754},\"players\": {\"max\": 10,\"online\": 5},\"description\": {\"text\": \"Minecraft Light Server Go\"}}"),
		).Pack(current.connection); err != nil {
			s.removePlayerAndExit(&current, err)
		}

		// Ping
		ping, err := current.getNextPacket()
		if err != nil {
			s.removePlayerAndExit(&current, err)
		}

		// Get Long payload of ping packet
		var pingPayload Long
		_, _ = pingPayload.ReadFrom(ping)

		// Pong (send ping payload)
		if err := NewPacket(handshakePong,
			pingPayload,
		).Pack(current.connection); err != nil {
			s.removePlayerAndExit(&current, err)
		}

		// End of status packet handling
		return
	} else { // State 2
		// Login start
		loginStart, err := current.getNextPacket()
		if err != nil {
			s.removePlayerAndExit(&current, err)
		}

		// Parse username
		_, _ = current.username.ReadFrom(loginStart)

		// Login success
		if loginStart.ID == handshakePacketID {
			if err := NewPacket(handshakeLoginSuccess,
				current.id,
				current.username,
			).Pack(current.connection); err != nil {
				s.removePlayerAndExit(&current, err)
			}

			s.addPlayer(&current)
		} else {
			s.removePlayerAndExit(&current, errors.New("invalid login packet id"))
		}
	}

	// Set Player initial parameters
	if err := current.writeJoinGame(); err != nil {
		s.removePlayerAndExit(&current, err)
	}
	if err := current.writePlayerPosition(
		current.x, current.y, current.z,
		current.yawAbs, current.pitchAbs,
		Byte(0x00), VarInt(current.int32FromUUID())); err != nil {
		s.removePlayerAndExit(&current, err)
	}
	if err := current.writeServerDifficulty(); err != nil {
		s.removePlayerAndExit(&current, err)
	}

	// Send 4 chunks to client
	chunks := [][]Int{{-1, 0}, {0, 0}, {-1, -1}, {0, -1}}
	for _, position := range chunks {
		if err := current.writeChunk(position[0], position[1]); err != nil {
			s.removePlayerAndExit(&current, err)
		}
	}

	// Send current player information to other connected clients
	s.broadcastPlayerInfo()
	s.broadcastChatMessage(string(current.username)+" joined the server", "Server")
	s.broadcastSpawnPlayer()

	// Start packets handler goroutine
	go s.handlePacket(&current)

	// Start KeepAlive goroutine
	go s.keepAliveUser(&current)
}

// handlePacket handles each packet sent by current client.
func (s *Server) handlePacket(p *Player) {
	for !p.isDeleted {
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
			if p.z != oldZ || coordinateToChunk(p.x) != coordinateToChunk(oldX) {
				if err := p.updateViewPosition(); err != nil {
					s.removePlayerAndExit(p, err)
				}
			}

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.int32FromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

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
			if p.z != oldZ || coordinateToChunk(p.x) != coordinateToChunk(oldX) {
				if err := p.updateViewPosition(); err != nil {
					s.removePlayerAndExit(p, err)
				}
			}

			// Send to other players
			s.broadcastPlayerPosAndLook(VarInt(p.int32FromUUID()), p.x, p.y, p.z, p.yaw, p.pitch, p.onGround)

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
			s.broadcastPlayerRotation(VarInt(p.int32FromUUID()), p.yaw, p.pitch, p.onGround)

		case readEntityActionPacketID:
			// Discard Entity ID
			_, _ = new(VarInt).ReadFrom(packet)

			var actionID VarInt
			if _, err := actionID.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			s.broadcastEntityAction(VarInt(p.int32FromUUID()), actionID)

		case readAnimationPacketID:
			var animationID VarInt
			if _, err := animationID.ReadFrom(packet); err != nil {
				s.removePlayerAndExit(p, err)
			}
			s.broadcastEntityAnimation(VarInt(p.int32FromUUID()), animationID)

		default:
			fmt.Printf("[%s] Unmanaged packet: 0x%02X\n", p.username, packet.ID)
		}
	}
}
