package MinecraftLightServer

import (
	"bytes"
	"errors"
	"io"
)

// Packet defines a Minecraft network data package
// +--------+-----------+------+
// | Length | Packet ID | Data |
// +--------+-----------+------+
type Packet struct {
	ID   int32
	data bytes.Buffer
}

// Pack prepares a packet and write it to w writer interface.
func (pk *Packet) Pack(w io.Writer) error {
	var packet bytes.Buffer
	id := VarInt(pk.ID)

	// Write total length
	if _, err := VarInt(id.Len() + pk.data.Len()).WriteTo(&packet); err != nil {
		return err
	}

	// Write packet id
	if _, err := id.WriteTo(&packet); err != nil {
		return err
	}

	// Write data
	if _, err := pk.data.WriteTo(&packet); err != nil {
		return err
	}

	// Send result to w
	if _, err := packet.WriteTo(w); err != nil {
		return err
	}

	return nil
}

// Unpack reads a packet from r reader interface.
func (pk *Packet) Unpack(r io.Reader) error {
	// Get packet length
	var length VarInt
	if _, err := length.ReadFrom(r); err != nil {
		return err
	} else if length < 1 {
		return errors.New("packet length too small")
	}

	// Save data
	buf := make([]byte, length)
	if _, err := r.Read(buf); err != nil {
		return errors.New("unable to read packet content: " + err.Error())
	}
	pk.data = *bytes.NewBuffer(buf)

	// Get packet id
	var packetID VarInt
	if _, err := packetID.ReadFrom(&pk.data); err != nil {
		return errors.New("unable to read packet id: " + err.Error())
	}
	pk.ID = int32(packetID)

	return nil
}

// Read implements io.Reader interface for Packet.
func (pk *Packet) Read(p []byte) (n int, err error) {
	return pk.data.Read(p)
}

// Write implements io.Writer interface for Packet.
func (pk *Packet) Write(p []byte) (n int, err error) {
	return pk.data.Write(p)
}

// NewPacket creates a new packet using input data.
func NewPacket(packetID int32, data ...io.WriterTo) *Packet {
	packet := new(Packet)
	packet.ID = packetID

	for _, currType := range data {
		_, _ = currType.WriteTo(packet)
	}

	return packet
}
