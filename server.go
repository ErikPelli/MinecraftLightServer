package MinecraftLightServer

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const serverPort = "25565" // default listen port

type Server struct {
	port       string
	players    sync.Map // key
	counter    int      // number of users online
	counterMut sync.Mutex
}

func NewServer() *Server {
	s := new(Server)
	s.port = serverPort
	return s
}

func (s *Server) SetPort(port string) {
	s.port = port
}

func (s *Server) Start() error {
	err := s.listen(serverPort)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) incrementCounter() {
	s.counterMut.Lock()
	s.counter++
	s.counterMut.Unlock()
}

func (s *Server) decrementCounter() {
	s.counterMut.Lock()
	if s.counter > 0 {
		s.counter--
	}
	s.counterMut.Unlock()
}

func (s *Server) keepAliveUser(current *Player) {
	for {
		// Keep Alive packet with random int
		random := Long(rand.Int63())
		keepAlive := NewPacket(keepAlivePacketID, random)

		// if there is a connection error remove client from players map
		if err := keepAlive.Pack(current.connection); err != nil {
			s.players.Delete(current.username)
			s.decrementCounter()

			fmt.Println("Client " + current.username + " has been disconnected")
			current.closeGoroutineAndConnection(err)
		}

		// send keep alive every 18 seconds (the maximum limit is 20 seconds)
		time.Sleep(time.Second * 18)
	}
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
			player.closeGoroutineAndConnection(err)
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

func (s *Server) broadcastPlayerPosAndLook(id VarInt, x, y, z Double, yaw, pitch Angle, onGround Boolean) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntfromUUID()) != id {
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
		if VarInt(player.getIntfromUUID()) != id {
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
		if VarInt(player.getIntfromUUID()) != id {
			_ = player.writeEntityAction(id, action)
		}

		return true
	})
}

func (s *Server) broadcastEntityAnimation(id VarInt, animation VarInt) {
	s.players.Range(func(key interface{}, playerInterface interface{}) bool {
		player := playerInterface.(*Player)

		// Don't send to current player
		if VarInt(player.getIntfromUUID()) != id {
			_ = player.writeEntityAnimation(id, animation)
		}

		return true
	})
}
