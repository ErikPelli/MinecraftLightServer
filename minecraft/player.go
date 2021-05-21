package minecraft

import (
	"errors"
	"net"
)

// Based on https://wiki.vg/Protocol
const(
	minecraftProtocol = 754
	handshakePacketID = 0x00
	keepAlivePacketID = 0x1F
)

type player struct {
	connection net.Conn
	id UUID
}

func(p *player) getNextPacket() *Packet {
	packet := new(Packet)
	err := packet.Unpack(p.connection)
	if err != nil {
		panic(err)
	}

	return packet
}

func (p *player) readHandshake() (state *VarInt, err error){
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