package MinecraftLightServer

import (
	"bytes"
	"errors"
	"net"
)

// Minecraft protocol and handshake constants.
const (
	minecraftProtocol     = 754
	handshakePacketID     = 0x00
	handshakePong         = 0x01
	handshakeLoginSuccess = 0x02
)

// Minecraft write packets (id).
const (
	spawnPlayerPacketID         = 0x04
	writeEntityAnimationID      = 0x05
	serverDifficultyPacketID    = 0x0D
	writeChatPacketID           = 0x0E
	keepAlivePacketID           = 0x1F
	writeChunkPacketID          = 0x20
	joinGamePacketID            = 0x24
	writeEntityRotationPacketID = 0x29
	broadcastPlayerInfoPacketID = 0x32
	playerPositionPacketID      = 0x34
	destroyEntityPacketID       = 0x36
	writeEntityLookPacketID     = 0x3A
	updateViewPacketID          = 0x40
	writeEntityMetadataPacketID = 0x44
	writeEntityTeleportPacketID = 0x56
)

// Minecraft read packets (id).
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

// Player is a single player that is currently in the server.
type Player struct {
	connection       net.Conn // TCP connection
	id               UUID     // random generated UUID
	isDeleted        bool     // has current user been deleted from server?
	username         String   // player username
	x, y, z          Double   // current coordinates of player
	yawAbs, pitchAbs Float    // absolute values of player visual in degrees
	yaw, pitch       Angle    // player visual expressed as an Angle (1/256)
	onGround         Boolean  // is the player on ground?
}

// getNextPacket gets next packet sent by current client.
func (p *Player) getNextPacket() (*Packet, error) {
	packet := new(Packet)
	err := packet.Unpack(p.connection)
	return packet, err
}

// readHandshake parses an handshake packet and check if its fields are valid.
func (p *Player) readHandshake(packet *Packet) (state *VarInt, err error) {
	// Protocol version
	version := new(VarInt)
	if _, err = version.ReadFrom(packet); err != nil {
		return
	} else if *version != minecraftProtocol {
		// Check minecraft protocol version
		err = errors.New("wrong protocol version")
	}

	// Discard server address and port
	_, _ = new(String).ReadFrom(packet)
	_, _ = new(UnsignedShort).ReadFrom(packet)

	// Next state
	state = new(VarInt)
	if _, err = state.ReadFrom(packet); err == nil {
		// if no error, check value
		if *state != 1 && *state != 2 {
			// Check next state value
			err = errors.New("wrong next state")
		}
	}

	return
}

// int32FromUUID converts player UUID to an int32.
func (p *Player) int32FromUUID() int32 {
	// 4 MSBs
	return int32(p.id[0])<<24 | int32(p.id[1])<<16 | int32(p.id[2])<<8 | int32(p.id[3])
}

// writeJoinGame sends world's settings to client.
func (p *Player) writeJoinGame() error {
	return NewPacket(joinGamePacketID,
		Int(p.int32FromUUID()),             // Entity ID
		Boolean(false),                     // Is hardcore
		UnsignedByte(0),                    // 0 = Survival mode
		Byte(-1),                           // previous gameplay
		VarInt(1),                          // there is only one world
		String("minecraft:overworld"),      // available world
		bytes.NewBuffer(dimensionCodecNBT), // world settings
		bytes.NewBuffer(dimensionNBT),      //
		String("minecraft:overworld"),      // player spawn world
		Long(0x123456789abcdef0),           // hashed seed
		VarInt(10),                         // max players
		VarInt(10),                         // rendering distance in chunks
		Boolean(false),                     // reduced debug info
		Boolean(false),                     // enable respawn screen
		Boolean(false),                     // is debug
		Boolean(true),                      // is flat
	).Pack(p.connection)
}

// writePlayerPosition sends specified coordinates to this player.
func (p *Player) writePlayerPosition(x, y, z Double, yawAbs, pitchAbs Float, flags Byte, teleportID VarInt) error {
	return NewPacket(playerPositionPacketID,
		x, y, z, // player coordinates
		yawAbs, pitchAbs, // player visual
		flags, teleportID, // parameters for client
	).Pack(p.connection)
}

// writeServerDifficulty sends current server difficulty to client.
func (p *Player) writeServerDifficulty() error {
	// Mode: peaceful, locked
	return NewPacket(serverDifficultyPacketID, UnsignedByte(0), Boolean(true)).Pack(p.connection)
}

// writeChunk sends a world chunk to the client.
func (p *Player) writeChunk(x, y Int) error {
	return NewPacket(writeChunkPacketID,
		x, y, // coordinates of chunk
		Boolean(true),                                    // full chunk
		VarInt(0x01),                                     // bit mask, blocks included in this data packet
		bytes.NewBuffer(heightMapNBT),                    // height map, highest blocks
		VarInt(1024),                                     // biome array length
		bytes.NewBuffer(bytes.Repeat([]byte{127}, 1024)), // void biome
		VarInt(4487),                                     // length of data
		// data start
		Short(256),               // non-air blocks
		UnsignedByte(8),          // bits per block
		VarInt(256),              // palette length
		bytes.NewBuffer(palette), // write palette
		VarInt(512),              // chunk length (512 long, 4096 bytes)
		bytes.NewBuffer(chunk),   // chunk bytes
		// data end
		VarInt(0), // number of block entities (zero)
	).Pack(p.connection)
}

// updateViewPosition sends to the player the chunk it is currently in.
func (p *Player) updateViewPosition() error {
	return NewPacket(updateViewPacketID,
		coordinateToChunk(p.x),
		coordinateToChunk(p.z),
	).Pack(p.connection)
}

// writeChatMessage sends a message to current player chat.
func (p *Player) writeChatMessage(msg, username string) error {
	return NewPacket(writeChatPacketID,
		String("{\"text\": \"<"+username+"> "+msg+"\",\"bold\": \"false\"}"),
		Byte(0),
		p.id,
	).Pack(p.connection)
}

// writeSpawnPlayer sends a spawn player packet to this client.
func (p *Player) writeSpawnPlayer(id VarInt, playerUUID UUID, x, y, z Double, yaw, pitch Angle) error {
	return NewPacket(spawnPlayerPacketID, id, playerUUID, x, y, z, yaw, pitch).Pack(p.connection)
}

// writeEntityTeleport changes position of a player and sends the packet to this client.
func (p *Player) writeEntityTeleport(x, y, z Double, yaw, pitch Angle, onGround Boolean, id VarInt) error {
	return NewPacket(writeEntityTeleportPacketID, id, x, y, z, yaw, pitch, onGround).Pack(p.connection)
}

// writeEntityLook changes visual of a player and sends the packet to this client.
func (p *Player) writeEntityLook(id VarInt, yaw Angle) error {
	return NewPacket(writeEntityLookPacketID, id, yaw).Pack(p.connection)
}

// writeEntityRotation rotates a player and sends the packet to this client.
func (p *Player) writeEntityRotation(id VarInt, yaw, pitch Angle, onGround Boolean) error {
	return NewPacket(writeEntityRotationPacketID, id, yaw, pitch, onGround).Pack(p.connection)
}

// writeEntityAction sends an action done by a player, specified by id, to this client.
func (p *Player) writeEntityAction(id VarInt, action VarInt) error {
	packet := NewPacket(writeEntityMetadataPacketID, id)

	switch action {
	case 0: // Start sneaking
		_, _ = UnsignedByte(6).WriteTo(packet) // field unique id
		_, _ = VarInt(18).WriteTo(packet)      // pose
		_, _ = VarInt(5).WriteTo(packet)       // sneak
	case 1: // Stop sneaking
		_, _ = UnsignedByte(6).WriteTo(packet) // field unique id
		_, _ = VarInt(18).WriteTo(packet)      // pose
		_, _ = VarInt(0).WriteTo(packet)       // stand up
	case 3: // Start sprinting
		_, _ = UnsignedByte(0).WriteTo(packet) // field unique id
		_, _ = VarInt(0).WriteTo(packet)       // byte
		_, _ = VarInt(0x08).WriteTo(packet)    // sprinting
	case 4: // Stop sprinting
		_, _ = UnsignedByte(0).WriteTo(packet) // field unique id
		_, _ = VarInt(0).WriteTo(packet)       // byte
		_, _ = VarInt(0).WriteTo(packet)       // no action
	default:
		// Do nothing if action isn't supported
		return nil
	}

	_, _ = UnsignedByte(0xFF).WriteTo(packet) // Terminate entity metadata array
	return packet.Pack(p.connection)
}

// writeEntityAnimation sends an action that produce an animation, done by a player,
// specified by id, to this client.
func (p *Player) writeEntityAnimation(id VarInt, animation VarInt) error {
	packet := NewPacket(writeEntityAnimationID, id)

	switch animation {
	case 0:
		_, _ = Byte(0).WriteTo(packet) // Main hand
	case 1:
		_, _ = Byte(3).WriteTo(packet) // Off hand
	}
	return packet.Pack(p.connection)
}
