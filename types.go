package MinecraftLightServer

import (
	"errors"
	"github.com/google/uuid"
	"io"
	"math"
)

// Minecraft packet field types
type (
	// Boolean type (true = 0x01, false = 0x00).
	Boolean bool
	// Byte is signed 8-bit integer, two's complement.
	Byte int8
	// UnsignedByte is unsigned 8-bit integer.
	UnsignedByte uint8
	// Short is signed 16-bit integer, two's complement.
	Short int16
	// UnsignedShort is unsigned 16-bit integer.
	UnsignedShort uint16
	// Int is signed 32-bit integer, two's complement.
	Int int32
	// Long is signed 64-bit integer, two's complement.
	Long int64
	// A Float is a single-precision 32-bit IEEE 754 floating point number.
	Float float32
	// A Double is a double-precision 64-bit IEEE 754 floating point number.
	Double float64
	// String is a sequence of Unicode values.
	String string

	// VarInt is variable-length data encoding a two's complement signed 32-bit integer.
	VarInt int32
	// VarLong is variable-length data encoding a two's complement signed 64-bit integer.
	VarLong int64

	// Angle is a rotation angle in steps of 1/256 of a full turn (360°).
	Angle Byte
	// UUID is an unsigned 128-bit integer.
	UUID uuid.UUID
)

// WriteTo encodes a Boolean.
func (b Boolean) WriteTo(w io.Writer) (int64, error) {
	var v byte
	if b {
		v = 0x01
	} else {
		v = 0x00
	}
	nn, err := w.Write([]byte{v})
	return int64(nn), err
}

// ReadFrom decodes a Boolean.
func (b *Boolean) ReadFrom(r io.Reader) (n int64, err error) {
	v, err := readByte(r)
	if err != nil {
		return 1, err
	}

	*b = v != 0
	return 1, nil
}

// WriteTo encodes a String.
func (s String) WriteTo(w io.Writer) (n int64, err error) {
	byteStr := []byte(s)
	n, err = VarInt(len(byteStr)).WriteTo(w)
	if err != nil {
		return
	}
	n2, err := w.Write(byteStr)
	n += int64(n2)
	return
}

// ReadFrom decodes a String.
func (s *String) ReadFrom(r io.Reader) (n int64, err error) {
	var l VarInt // String length

	nn, err := l.ReadFrom(r)
	if err != nil {
		return nn, err
	}
	n += nn

	bs := make([]byte, l)
	if _, err := io.ReadFull(r, bs); err != nil {
		return n, err
	}
	n += int64(l)

	*s = String(bs)
	return n, nil
}

// readByte read one byte from io.Reader
func readByte(r io.Reader) (byte, error) {
	if r, ok := r.(io.ByteReader); ok {
		return r.ReadByte()
	}
	var v [1]byte
	_, err := io.ReadFull(r, v[:])
	return v[0], err
}

// WriteTo encodes a Byte.
func (b Byte) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write([]byte{byte(b)})
	return int64(nn), err
}

// ReadFrom decodes a Byte.
func (b *Byte) ReadFrom(r io.Reader) (n int64, err error) {
	v, err := readByte(r)
	if err != nil {
		return 0, err
	}
	*b = Byte(v)
	return 1, nil
}

// WriteTo encodes an UnsignedByte.
func (u UnsignedByte) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write([]byte{byte(u)})
	return int64(nn), err
}

// ReadFrom decodes an UnsignedByte.
func (u *UnsignedByte) ReadFrom(r io.Reader) (n int64, err error) {
	v, err := readByte(r)
	if err != nil {
		return 0, err
	}
	*u = UnsignedByte(v)
	return 1, nil
}

// WriteTo encodes a Short.
func (s Short) WriteTo(w io.Writer) (int64, error) {
	n := uint16(s)
	nn, err := w.Write([]byte{byte(n >> 8), byte(n)})
	return int64(nn), err
}

// ReadFrom decodes a Short.
func (s *Short) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*s = Short(int16(bs[0])<<8 | int16(bs[1]))
	return
}

// WriteTo encodes an UnsignedShort.
func (us UnsignedShort) WriteTo(w io.Writer) (int64, error) {
	n := uint16(us)
	nn, err := w.Write([]byte{byte(n >> 8), byte(n)})
	return int64(nn), err
}

// ReadFrom decodes an UnsignedShort.
func (us *UnsignedShort) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*us = UnsignedShort(int16(bs[0])<<8 | int16(bs[1]))
	return
}

// WriteTo encodes an Int.
func (i Int) WriteTo(w io.Writer) (int64, error) {
	n := uint32(i)
	nn, err := w.Write([]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	return int64(nn), err
}

// ReadFrom decodes an Int.
func (i *Int) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [4]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*i = Int(int32(bs[0])<<24 | int32(bs[1])<<16 | int32(bs[2])<<8 | int32(bs[3]))
	return
}

// WriteTo encodes a Long.
func (l Long) WriteTo(w io.Writer) (int64, error) {
	n := uint64(l)
	nn, err := w.Write([]byte{
		byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	})
	return int64(nn), err
}

// ReadFrom decodes a Long.
func (l *Long) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [8]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*l = Long(int64(bs[0])<<56 | int64(bs[1])<<48 | int64(bs[2])<<40 | int64(bs[3])<<32 |
		int64(bs[4])<<24 | int64(bs[5])<<16 | int64(bs[6])<<8 | int64(bs[7]))
	return
}

// WriteTo encodes a VarInt.
func (v VarInt) WriteTo(w io.Writer) (n int64, err error) {
	var vi = make([]byte, 0, 5)
	num := uint32(v)
	for {
		b := num & 0x7F
		num >>= 7
		if num != 0 {
			b |= 0x80
		}
		vi = append(vi, byte(b))
		if num == 0 {
			break
		}
	}
	nn, err := w.Write(vi)
	return int64(nn), err
}

// ReadFrom decodes a VarInt.
func (v *VarInt) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint32
	for sec := byte(0x80); sec&0x80 != 0; n++ {
		if n > 5 {
			return n, errors.New("VarInt is too big")
		}

		sec, err = readByte(r)
		if err != nil {
			return n, err
		}

		V |= uint32(sec&0x7F) << uint32(7*n)
	}

	*v = VarInt(V)
	return
}

// WriteTo encodes a VarLong.
func (v VarLong) WriteTo(w io.Writer) (n int64, err error) {
	var vi = make([]byte, 0, 10)
	num := uint64(v)
	for {
		b := num & 0x7F
		num >>= 7
		if num != 0 {
			b |= 0x80
		}
		vi = append(vi, byte(b))
		if num == 0 {
			break
		}
	}
	nn, err := w.Write(vi)
	return int64(nn), err
}

// ReadFrom decodes a VarLong.
func (v *VarLong) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint64
	for sec := byte(0x80); sec&0x80 != 0; n++ {
		if n >= 10 {
			return n, errors.New("VarLong is too big")
		}
		sec, err = readByte(r)
		if err != nil {
			return
		}

		V |= uint64(sec&0x7F) << uint64(7*n)
	}

	*v = VarLong(V)
	return
}

// WriteTo encodes an Angle.
func (a Angle) WriteTo(w io.Writer) (int64, error) {
	return Byte(a).WriteTo(w)
}

// ReadFrom decodes an Angle.
func (a *Angle) ReadFrom(r io.Reader) (int64, error) {
	return (*Byte)(a).ReadFrom(r)
}

// WriteTo encodes a Float.
func (f Float) WriteTo(w io.Writer) (n int64, err error) {
	return Int(math.Float32bits(float32(f))).WriteTo(w)
}

// ReadFrom decodes a Float.
func (f *Float) ReadFrom(r io.Reader) (n int64, err error) {
	var v Int

	n, err = v.ReadFrom(r)
	if err != nil {
		return
	}

	*f = Float(math.Float32frombits(uint32(v)))
	return
}

// ToAngle converts Float angle (0-360) to Angle (0-256).
func (f Float) toAngle() Angle {
	return Angle(math.Floor(float64(f) / 360 * 256))
}

// WriteTo encodes a Double.
func (d Double) WriteTo(w io.Writer) (n int64, err error) {
	return Long(math.Float64bits(float64(d))).WriteTo(w)
}

// ReadFrom decodes a Double.
func (d *Double) ReadFrom(r io.Reader) (n int64, err error) {
	var v Long
	n, err = v.ReadFrom(r)
	if err != nil {
		return
	}

	*d = Double(math.Float64frombits(uint64(v)))
	return
}

// WriteTo encodes an UUID.
func (u UUID) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write(u[:])
	return int64(nn), err
}

// ReadFrom decodes an UUID.
func (u *UUID) ReadFrom(r io.Reader) (n int64, err error) {
	nn, err := io.ReadFull(r, (*u)[:])
	return int64(nn), err
}

// coordinateToChunk convert an absolute double coordinate to a chunk coordinate.
func coordinateToChunk(coordinate Double) VarInt {
	coordinate /= 16
	if coordinate < 0 {
		coordinate -= 1
	}
	return VarInt(coordinate)
}
