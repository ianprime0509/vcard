package vcard

import (
	"io"
)

// UnfoldingReader is a Reader that unfolds lines of text as they are
// encountered and converts the "\r\n" line ending sequence to a single '\n'.
type UnfoldingReader struct {
	r      io.Reader
	line   int
	unread []byte // a stack of bytes that are queued up to be read
	peeked int    // if not -1, the byte that was peeked at
}

// NewUnfoldingReader returns a new UnfoldingReader wrapping the given Reader.
func NewUnfoldingReader(r io.Reader) *UnfoldingReader {
	return &UnfoldingReader{
		r:      r,
		line:   1,
		peeked: -1,
	}
}

// Read implements io.Reader for UnfoldingReader.
func (r *UnfoldingReader) Read(bs []byte) (int, error) {
	var i int
	for i = range bs {
		b, err := r.ReadByte()
		if err != nil {
			return i, err
		}
		bs[i] = b
	}
	return i, nil
}

// ReadByte reads a single byte from the reader.
func (r *UnfoldingReader) ReadByte() (byte, error) {
	if b := r.peeked; b != -1 {
		r.peeked = -1
		return byte(b), nil
	}
	if len(r.unread) > 0 {
		b := r.unread[len(r.unread)-1]
		r.unread = r.unread[:len(r.unread)-1]
		return b, nil
	}

	b, err := r.readByte()
	if err != nil {
		return 0, err
	}
	if b == '\r' {
		b2, err := r.readByte()
		if err != nil {
			return '\r', nil
		}
		if b2 == '\n' {
			r.line++
			b3, err := r.readByte()
			if err != nil {
				return '\n', nil
			}
			if b3 == ' ' || b3 == '\t' {
				return r.ReadByte()
			}
		}
		r.unread = append(r.unread, b2)
	} else if b == '\n' {
		r.line++
		b2, err := r.readByte()
		if err != nil {
			return '\n', nil
		}
		if b2 == ' ' || b2 == '\t' {
			return r.ReadByte()
		}
		r.unread = append(r.unread, b2)
	}
	return b, nil
}

// readByte reads a single byte from the underlying reader (for implementation
// convenience).
func (r *UnfoldingReader) readByte() (byte, error) {
	var bs [1]byte
	n, err := r.r.Read(bs[:])
	if n == 0 {
		return 0, err
	}
	return bs[0], nil
}

// PeekByte reads the next byte but keeps it for a future call to ReadByte.
func (r *UnfoldingReader) PeekByte() (byte, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	r.peeked = int(b)
	return b, nil
}

// Line returns the number of the current line being read.
func (r *UnfoldingReader) Line() int {
	return r.line
}
