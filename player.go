package MinecraftLightServer

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"runtime"
)

// Based on https://wiki.vg/Protocol
const (
	minecraftProtocol           = 754
	handshakePacketID           = 0x00
	handshakePong               = 0x01
	handshakeLoginSuccess       = 0x02
	keepAlivePacketID           = 0x1F
	joinGamePacketID            = 0x24
	PlayerPositionPacketID      = 0x34
	serverDifficultyPacketID    = 0x0D
	chunkPacketID               = 0x20
	broadcastPlayerInfoPacketID = 0x32
	writeChatPacketID           = 0x0E
	writeEntityLookPacketID     = 0x3A
	spawnPlayerPacketID         = 0x04
)

type Player struct {
	connection       net.Conn
	id               UUID
	x, y, z          Double
	yawAbs, pitchAbs Float // absolute values
	yaw, pitch       Angle
}

func (p *Player) getNextPacket() (*Packet, error) {
	packet := new(Packet)
	err := packet.Unpack(p.connection)
	return packet, err
}

func (p *Player) closeGoroutineAndConnection(err error) {
	fmt.Println(err)
	_ = p.connection.Close()
	runtime.Goexit()
}

func (p *Player) getIntfromUUID() Int {
	playerId := int32(p.id[0])<<24 | int32(p.id[1])<<16 | int32(p.id[2])<<8 | int32(p.id[3])
	return Int(playerId)
}

func (p *Player) readHandshake() (state *VarInt, err error) {
	packet, err := p.getNextPacket()
	if err != nil {
		return
	}

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
	join := NewPacket(joinGamePacketID,
		p.getIntfromUUID(),                 // Entity ID
		Boolean(false),                     // Is hardcore
		UnsignedByte(0),                    // Survival mode
		Byte(-1),                           // previous gameplay
		VarInt(1),                          // only one world
		String("minecraft:overworld"),      // available world
		bytes.NewBuffer(dimensionCodecNBT), // dimension codec
		bytes.NewBuffer(dimensionNBT),      // dimension
		String("minecraft:overworld"),      // spawn world
		Long(0x123456789abcdef0),           // hashed seed
		VarInt(10),                         // max players
		VarInt(15),                         // rendering distance
		Boolean(false),                     // reduced debug info
		Boolean(false),                     // enable respawn screen
		Boolean(false),                     // is debug
		Boolean(true),                      // is flat
	)

	return join.Pack(p.connection)
}

func (p *Player) writePlayerPosition(x, y, z Double, yawAbs, pitchAbs Float, flags Byte, teleportID VarInt) error {
	position := NewPacket(PlayerPositionPacketID,
		x, y, z, // coordinates
		yawAbs, pitchAbs, // visual
		flags, teleportID,
	)

	return position.Pack(p.connection)
}

func (p *Player) writeServerDifficulty() error {
	// locked peaceful mode
	difficult := NewPacket(serverDifficultyPacketID, UnsignedByte(0), Boolean(true))
	return difficult.Pack(p.connection)
}

func (p *Player) writeChunk(x, y Int) error {
	chunk := NewPacket(chunkPacketID,
		x, y,
		Boolean(true), // full chunk
		VarInt(0x01),  // bit mask, blocks included in this data packet
		bytes.NewBuffer(heightMapNBT),
		VarInt(1024),                                     // biome array length
		bytes.NewBuffer(bytes.Repeat([]byte{127}, 1024)), // void biome
		VarInt(4487),                                     // length of data
		// data start
		Short(1),                 // non-air blocks, client doesn't need it
		UnsignedByte(8),          // bits per block
		VarInt(256),              // palette length
		bytes.NewBuffer(palette), // write palette
		VarInt(512),              // chunk length (512 long = 4096 bytes)
		bytes.NewBuffer(chunk[x][y]),
		// data end
		VarInt(0), // number of block entities (zero)
	)
	return chunk.Pack(p.connection)
}

func (p *Player) writeChat(msg, username string) error {
	chat := NewPacket(writeChatPacketID,
		String("{\"text\": \"<"+username+"> "+msg+"\",\"bold\": \"false\"}"),
		Byte(0),
		p.id,
	)
	return chat.Pack(p.connection)
}

func (p *Player) writeEntityLook(id VarInt, yaw Angle) error {
	look := NewPacket(writeEntityLookPacketID, id, yaw)
	return look.Pack(p.connection)
}

func (p *Player) writeSpawnPlayer(id VarInt, playerUUID UUID, x, y, z Double, yaw, pitch Angle) error {
	spawn := NewPacket(spawnPlayerPacketID, id, playerUUID, x, y, z, yaw, pitch)
	return spawn.Pack(p.connection)
}
