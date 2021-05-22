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
	minecraftProtocol     = 754
	handshakePacketID     = 0x00
	handshakePong         = 0x01
	handshakeLoginSuccess = 0x02
)

// Write packets
const (
	spawnPlayerPacketID         = 0x04
	writeEntityAnimationID		= 0x05
	serverDifficultyPacketID    = 0x0D
	writeChatPacketID           = 0x0E
	keepAlivePacketID           = 0x1F
	writeChunkPacketID          = 0x20
	joinGamePacketID            = 0x24
	writeEntityRotationPacketID = 0x29
	broadcastPlayerInfoPacketID = 0x32
	PlayerPositionPacketID      = 0x34
	writeEntityLookPacketID     = 0x3A
	writeEntityMetadataPacketID = 0x44
	writeEntityTeleportPacketID = 0x56
)

// Read packets
const (
	readTeleportConfirmPacketID = 0x00
	readChatPacketID            = 0x03
	readKeepAlivePacketID       = 0x10
	readPositionPacketID        = 0x12
	readPositionAndLookPacketID = 0x13
	readRotationPacketID        = 0x14
	readEntityActionPacketID    = 0x1C
	readAnimationPacketID       = 0x2C
)

type Player struct {
	connection       net.Conn
	id               UUID
	username         String
	x, y, z          Double
	yawAbs, pitchAbs Float // absolute values
	yaw, pitch       Angle
	onGround         Boolean
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
		VarInt(10),                         // rendering distance
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
	chunk := NewPacket(writeChunkPacketID,
		x, y,
		Boolean(true), 											// full chunk
		VarInt(0x01),  											// bit mask, blocks included in this data packet
		bytes.NewBuffer(heightMapNBT),
		VarInt(1024),                                     		// biome array length
		bytes.NewBuffer(bytes.Repeat([]byte{127}, 1024)), // void biome
		VarInt(4487),                                     		// length of data
		// data start
		Short(1),                 								// non-air blocks, client doesn't need it
		UnsignedByte(8),          								// bits per block
		VarInt(256),              								// palette length
		bytes.NewBuffer(palette),								// write palette
		VarInt(512), 											// chunk length (512 long = 4096 bytes)
		bytes.NewBuffer(chunk),
		// data end
		VarInt(0), 												// number of block entities (zero)
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

func (p *Player) writeSpawnPlayer(id VarInt, playerUUID UUID, x, y, z Double, yaw, pitch Angle) error {
	return NewPacket(spawnPlayerPacketID, id, playerUUID, x, y, z, yaw, pitch).Pack(p.connection)
}

func (p *Player) writeEntityTeleport(x, y, z Double, yaw, pitch Angle, onGround Boolean, id VarInt) error {
	return NewPacket(writeEntityTeleportPacketID, id, x, y, z, yaw, pitch, onGround).Pack(p.connection)
}

func (p *Player) writeEntityLook(id VarInt, yaw Angle) error {
	return NewPacket(writeEntityLookPacketID, id, yaw).Pack(p.connection)
}

func (p *Player) writeEntityRotation(id VarInt, yaw, pitch Angle, onGround Boolean) error {
	return NewPacket(writeEntityRotationPacketID, id, yaw, pitch, onGround).Pack(p.connection)
}

func (p *Player) writeEntityAction(id VarInt, action VarInt) error {
	packet := NewPacket(writeEntityMetadataPacketID, id)

	switch action {
	case 0: // start sneaking
		_, _ = UnsignedByte(6).WriteTo(packet) // field unique id
		_, _ = VarInt(18).WriteTo(packet)      // pose
		_, _ = VarInt(5).WriteTo(packet)       // sneak
	case 1: // stop sneaking
		_, _ = UnsignedByte(6).WriteTo(packet) // field unique id
		_, _ = VarInt(18).WriteTo(packet)      // pose
		_, _ = VarInt(0).WriteTo(packet)       // stand up
	case 3: // start sprinting
		_, _ = UnsignedByte(0).WriteTo(packet) // field unique id
		_, _ = VarInt(0).WriteTo(packet)       // byte
		_, _ = VarInt(0x08).WriteTo(packet)    // sprinting
	case 4: // stop sprinting
		_, _ = UnsignedByte(0).WriteTo(packet) // field unique id
		_, _ = VarInt(0).WriteTo(packet)       // byte
		_, _ = VarInt(0).WriteTo(packet)       // no action
	default:
		// Do nothing if action isn't supported
		return nil
	}

	_, _ = UnsignedByte(0xFF).WriteTo(packet) // terminate entity metadata array
	return packet.Pack(p.connection)
}

func (p *Player) writeEntityAnimation(id VarInt, animation VarInt) error {
	packet := NewPacket(writeEntityAnimationID, id)

	switch animation {
	case 0:
		_, _ = Byte(0).WriteTo(packet) // main hand
	case 1:
		_, _ = Byte(3).WriteTo(packet) // off hand
	}
	return packet.Pack(p.connection)
}