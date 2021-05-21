package minecraft

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
	Data bytes.Buffer
}

// Pack prepares a packet and write it to w writer interface.
func (pk *Packet) Pack(w io.Writer) error {
	// Write packet id to buffer
	var id bytes.Buffer
	if _, err := VarInt(pk.ID).WriteTo(&id); err != nil {
		panic(err)
	}

	// Total length
	if _, err := VarInt(id.Len() + pk.Data.Len()).WriteTo(w); err != nil {
		return err
	}
	// Packet id
	if _, err := id.WriteTo(w); err != nil {
		return err
	}
	// Data
	if _, err := pk.Data.WriteTo(w); err != nil {
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
	}
	if length < 1 {
		return errors.New("packet length too small")
	}

	// Save data
	buf := make([]byte, length)
	if _, err := r.Read(buf); err != nil {
		return errors.New("unable to read packet content: " + err.Error())
	}
	pk.Data = *bytes.NewBuffer(buf)

	// Get packet id
	var packetID VarInt
	if _, err := packetID.ReadFrom(&pk.Data); err != nil {
		return errors.New("unable to read packet id: " + err.Error())
	}
	pk.ID = int32(packetID)

	return nil
}

// Read implements io.Reader interface for Packet
func(pk *Packet) Read(p []byte) (n int, err error) {
	return pk.Data.Read(p)
}

// Write implements io.Writer interface for Packet
func(pk *Packet) Write(p []byte) (n int, err error) {
	return pk.Data.Write(p)
}

// IncludeToPacket appends data to current Packet.
func NewPacket(packetid int32, data []io.WriterTo) *Packet{
	packet := new(Packet)
	packet.ID = packetid

	for _, currType := range data{
		_, _ = currType.WriteTo(packet)
	}

	return packet
}