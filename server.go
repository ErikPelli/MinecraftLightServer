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
// Leave portNumber empty to use default port (25565).
func NewServer(portNumber string) *Server {
	s := new(Server)

	if portNumber == "" {
		portNumber = serverPort
	}

	s.listener.port = portNumber
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
	close(s.listener.portValue)
	s.players.Range(func(key interface{}, value interface{}) bool {
		s.removePlayer(value.(*Player), errors.New("server closed"))
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

		// if there is a connection error remove client from players map
		if err := keepAlive.Pack(p.connection); err != nil {
			s.removePlayer(p, err)
		}

		// send keep alive every 18 seconds (the maximum limit is 20 seconds)
		time.Sleep(time.Second * 18)
	}
}

// addPlayer add a player and removes players
// actually connected with same username.
func (s *Server) addPlayer(p *Player) {
	precedent, ok := s.players.LoadAndDelete(p.username)
	if ok {
		// Close the connection if not already done
		_ = precedent.(*Player).connection.Close()
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
	// Close the connection if not already done
	_ = p.connection.Close()

	// Remove player from players map
	if _, ok := s.players.LoadAndDelete(p.username); ok {
		// log error
		fmt.Println("Client " + string(p.username) + " has been removed due to [" + err.Error() + "]")

		// Decrement players counter if deleted
		s.counterMut.Lock()
		s.counter--
		s.counterMut.Unlock()
	}

	// Exit current player goroutine
	runtime.Goexit()
}

func (s *Server) broadcastPlayerInfo() {
	s.players.Range(func(key interface{}, currentPlayer interface{}) bool {
		// Send packet to current host
		broadcast := NewPacket(broadcastPlayerInfoPacketID,
			VarInt(0),         // add player
			VarInt(s.counter), // number of players
		)

		s.players.Range(func(key interface{}, value interface{}) bool {
			// Add every player to packet
			_, _ = value.(*Player).id.WriteTo(broadcast)       // player uuid
			_, _ = value.(*Player).username.WriteTo(broadcast) // username
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

func (s *Server) broadcastChatMessage(msg, username string) {
	s.players.Range(func(key interface{}, value interface{}) bool {
		player := value.(*Player)
		if err := player.writeChat(msg, username); err != nil {
			s.removePlayer(player, err)
		}
		return true
	})

	fmt.Println("Broadcast chat message: <" + username + "> " + msg)
}

func (s *Server) broadcastSpawnPlayer() {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		currentPlayer := playerInterface.(*Player)
		s.players.Range(func(key interface{}, p interface{}) bool {
			// Get all other players
			otherPlayer := p.(*Player)
			if currentPlayer.id != otherPlayer.id {
				_ = currentPlayer.writeSpawnPlayer(
					VarInt(otherPlayer.getIntFromUUID()),
					otherPlayer.id,
					otherPlayer.x,
					otherPlayer.y,
					otherPlayer.z,
					otherPlayer.yaw,
					otherPlayer.pitch,
				)
				_ = currentPlayer.writeEntityLook(
					VarInt(otherPlayer.getIntFromUUID()),
					otherPlayer.yaw,
				)
			}
			return true
		})
		return true
	})
}

func (s *Server) broadcastPlayerPosAndLook(id VarInt, x, y, z Double, yaw, pitch Angle, onGround Boolean) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntFromUUID()) != id {
			_ = player.writeEntityTeleport(x, y, z, yaw, pitch, onGround, id)
			_ = player.writeEntityLook(id, yaw)
		}

		return true
	})
}

func (s *Server) broadcastPlayerRotation(id VarInt, yaw, pitch Angle, onGround Boolean) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntFromUUID()) != id {
			_ = player.writeEntityRotation(id, yaw, pitch, onGround)
			_ = player.writeEntityLook(id, yaw)
		}

		return true
	})
}

func (s *Server) broadcastEntityAction(id VarInt, action VarInt) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntFromUUID()) != id {
			_ = player.writeEntityAction(id, action)
		}

		return true
	})
}

func (s *Server) broadcastEntityAnimation(id VarInt, animation VarInt) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntFromUUID()) != id {
			_ = player.writeEntityAnimation(id, animation)
		}

		return true
	})
}
