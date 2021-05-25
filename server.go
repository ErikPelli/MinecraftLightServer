package MinecraftLightServer

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

const serverPort = "25565" // default listen port

// Server is a running Minecraft server.
type Server struct {
	listener struct { // listening port handling
		port      string      // current listening port
		portValue chan string // send port to listening function
		err       chan error  // get errors
	}
	players    sync.Map   // map of players online
	counter    int        // number of players online
	counterMut sync.Mutex // mutex for players counter
}

// NewServer creates a new Server using default port.
// portNumber is an optional argument and you have to leave
// it empty to use default port (25565).
func NewServer(portNumber ...string) *Server {
	s := new(Server)

	if len(portNumber) == 0 {
		s.listener.port = serverPort
	} else {
		s.listener.port = portNumber[0]
	}

	s.listener.portValue = make(chan string)
	s.listener.err = make(chan error)
	return s
}

// Start starts the server using the current port.
func (s *Server) Start() error {
	go s.listen(s.listener.portValue, s.listener.err)
	s.listener.portValue <- s.listener.port
	return <-s.listener.err
}

// SetPort changes port of the Minecraft server.
// Use it when server is running.
func (s *Server) SetPort(portNumber string) error {
	s.listener.portValue <- portNumber
	return <-s.listener.err
}

// Close stops the server and close its components.
func (s *Server) Close() error {
	// Close port changer channel
	close(s.listener.portValue)

	// Remove each of the connected clients
	s.players.Range(func(key interface{}, value interface{}) bool {
		s.removePlayerAndExit(value.(*Player), errors.New("server closed"))
		return true
	})
	return nil
}

// keepAliveUser sends keepalive packet to current player.
// This function must be started within a new goroutine.
func (s *Server) keepAliveUser(p *Player) {
	for {
		// Keep Alive packet with random int
		random := Long(rand.Int63())
		keepAlive := NewPacket(keepAlivePacketID, random)

		// If there is a connection error remove client from players map
		if err := keepAlive.Pack(p.connection); err != nil {
			if p.isDeleted {
				// Stop keepalive if user has been deleted
				break
			} else {
				// If there is an error and player hasn't
				// yet been deleted, delete him
				s.removePlayerAndExit(p, err)
			}
		}

		// send keep alive every 18 seconds (the maximum limit is 20 seconds)
		time.Sleep(time.Second * 18)
	}
}

// addPlayer add a player and removes players
// actually connected with same username.
func (s *Server) addPlayer(p *Player) {
	precedent, ok := s.players.Load(p.username)
	if ok {
		// Remove old player
		s.removePlayer(precedent.(*Player), errors.New("new player with same username"))
	} else {
		// Increment players counter if the player is new
		s.counterMut.Lock()
		s.counter++
		s.counterMut.Unlock()
	}
	s.players.Store(p.username, p)
}

// removePlayer removes a player from current Server.
// must be invoked by the player's handler goroutine.
func (s *Server) removePlayer(p *Player, err error) {
	p.isDeleted = true
	_ = p.connection.Close()

	// Remove player from players map
	if _, ok := s.players.LoadAndDelete(p.username); ok {
		// Log error
		fmt.Println("Client " + string(p.username) + " has been removed due to [" + err.Error() + "]")

		// Decrement players counter if deleted
		s.counterMut.Lock()
		s.counter--
		s.counterMut.Unlock()

		// Remove player from other clients
		s.players.Range(func(key interface{}, value interface{}) bool {
			currentPlayer := value.(*Player)

			_ = NewPacket(broadcastPlayerInfoPacketID,
				VarInt(4), // remove player
				VarInt(1), // number of players
				p.id,      // uuid
			).Pack(currentPlayer.connection)

			_ = NewPacket(destroyEntityPacketID,
				VarInt(1),                 // number of players
				VarInt(p.int32FromUUID()), // uuid
			).Pack(currentPlayer.connection)

			return true
		})
	}
}

// removePlayerAndExit removes a player from current Server
// and stops current goroutine.
func (s *Server) removePlayerAndExit(p *Player, err error) {
	s.removePlayer(p, err)
	runtime.Goexit()
}

// broadcastPlayerInfo sends to all players the current players connected,
// to use when a new user needs to be added.
func (s *Server) broadcastPlayerInfo() {
	s.players.Range(func(key interface{}, currentPlayer interface{}) bool {
		// Send packet to current host
		broadcast := NewPacket(broadcastPlayerInfoPacketID,
			VarInt(0),         // add player
			VarInt(s.counter), // number of players
		)

		// Add every player to packet
		s.players.Range(func(key interface{}, value interface{}) bool {
			currentPlayer := value.(*Player)

			_, _ = currentPlayer.id.WriteTo(broadcast)       // player uuid
			_, _ = currentPlayer.username.WriteTo(broadcast) // username
			_, _ = VarInt(0).WriteTo(broadcast)                // no properties
			_, _ = VarInt(0).WriteTo(broadcast)                // gamemode 0 (survival)
			_, _ = VarInt(123).WriteTo(broadcast)              // hardcoded ping
			_, _ = Boolean(false).WriteTo(broadcast)           // has display name
			return true
		})

		// Send players packet
		_ = broadcast.Pack(currentPlayer.(*Player).connection)
		return true
	})
}

// broadcastChatMessage sends a chat message to all connected players.
// msg is the message string and username is the sender.
func (s *Server) broadcastChatMessage(msg, username string) {
	s.players.Range(func(key interface{}, value interface{}) bool {
		player := value.(*Player)
		if err := player.writeChatMessage(msg, username); err != nil {
			s.removePlayerAndExit(player, err)
		}
		return true
	})

	fmt.Println("Broadcast chat message: <" + username + "> " + msg)
}

// broadcastSpawnPlayer sends the position of all other players to every client.
func (s *Server) broadcastSpawnPlayer() {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		currentPlayer := playerInterface.(*Player)
		s.players.Range(func(key interface{}, p interface{}) bool {
			// Get all other players
			otherPlayer := p.(*Player)
			if currentPlayer.id != otherPlayer.id {
				_ = currentPlayer.writeSpawnPlayer(
					VarInt(otherPlayer.int32FromUUID()),
					otherPlayer.id,
					otherPlayer.x,
					otherPlayer.y,
					otherPlayer.z,
					otherPlayer.yaw,
					otherPlayer.pitch,
				)
				_ = currentPlayer.writeEntityLook(
					VarInt(otherPlayer.int32FromUUID()),
					otherPlayer.yaw,
				)
			}
			return true
		})
		return true
	})
}

// broadcastPlayerPosAndLook sends to all other clients the position and the view of a player.
func (s *Server) broadcastPlayerPosAndLook(id VarInt, x, y, z Double, yaw, pitch Angle, onGround Boolean) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.int32FromUUID()) != id {
			_ = player.writeEntityTeleport(x, y, z, yaw, pitch, onGround, id)
			_ = player.writeEntityLook(id, yaw)
		}

		return true
	})
}

// broadcastPlayerPosAndLook sends to all other clients the rotation and the look of a player.
func (s *Server) broadcastPlayerRotation(id VarInt, yaw, pitch Angle, onGround Boolean) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.int32FromUUID()) != id {
			_ = player.writeEntityRotation(id, yaw, pitch, onGround)
			_ = player.writeEntityLook(id, yaw)
		}

		return true
	})
}

// broadcastEntityAction sends to all other clients an action of a player.
func (s *Server) broadcastEntityAction(id VarInt, action VarInt) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.int32FromUUID()) != id {
			_ = player.writeEntityAction(id, action)
		}

		return true
	})
}

// broadcastEntityAnimation sends to all other clients an animation of a player.
func (s *Server) broadcastEntityAnimation(id VarInt, animation VarInt) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.int32FromUUID()) != id {
			_ = player.writeEntityAnimation(id, animation)
		}

		return true
	})
}
