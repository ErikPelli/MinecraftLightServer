package MinecraftLightServer

import (
	"bytes"
	"errors"
	"net"
)

// Based on https://wiki.vg/Protocol
const (
	minecraftProtocol     = 754
	handshakePacketID     = 0x00
	handshakePong         = 0x01
	handshakeLoginSuccess = 0x02
	keepAlivePacketID     = 0x1F
	joinGamePacketID      = 0x24
)

type Player struct {
	connection net.Conn
	id         UUID
}

func (p *Player) getNextPacket() *Packet {
	packet := new(Packet)
	err := packet.Unpack(p.connection)
	if err != nil {
		panic(err)
	}

	return packet
}

func (p *Player) panicAndCloseConnection(err error) {
	_ = p.connection.Close()
	panic(err)
}

func (p *Player) getIntfromUUID() Int {
	playerId := int32(p.id[0])<<24 | int32(p.id[1])<<16 | int32(p.id[2])<<8 | int32(p.id[3])
	return Int(playerId)
}

func (p *Player) readHandshake() (state *VarInt, err error) {
	packet := p.getNextPacket()

	// Protocol version
	version := new(VarInt)
	if _, err = version.ReadFrom(packet); err != nil {
		return
	}

	// Discard server address and port
	_, _ = new(String).ReadFrom(packet)
	_, _ = new(UnsignedShort).ReadFrom(packet)

	// Next state
	state = new(VarInt)
	if _, err = state.ReadFrom(packet); err != nil {
		return
	}

	if packet.ID != handshakePacketID {
		err = errors.New("wrong packet id")
	} else if *version != minecraftProtocol {
		err = errors.New("wrong protocol version")
	} else if *state != 1 && *state != 2 {
		err = errors.New("wrong next state")
	}

	return
}

func (p *Player) joinGame() error {
	join := NewPacket(joinGamePacketID, []PacketFieldWrite{
		p.getIntfromUUID(),            // Entity ID
		Boolean(false),                // Is hardcore
		UnsignedByte(0),               // Survival mode
		Byte(-1),                      // previous gameplay
		VarInt(1),                     // only one world
		String("minecraft:overworld"), // available world
		bytes.NewBuffer(dimensionCodecNBT), // dimension codec
		bytes.NewBuffer(dimensionNBT), // dimension
		String("minecraft:overworld"), // spawn world
		Long(0x123456789abcdef0),      // hashed seed
		VarInt(10),                    // max players
		VarInt(15),                    // rendering distance
		Boolean(false),                // reduced debug info
		Boolean(false),                // enable respawn screen
		Boolean(false),                // is debug
		Boolean(true),                 // is flat
	})

	return join.Pack(p.connection)
}

func (p *Player) writePlayerPositiob
