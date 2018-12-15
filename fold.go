// Copyright 2018 Ian Johnson
//
// This file is part of vcard. Vcard is free software: you are free to use it
// for any purpose, make modified versions and share it with others, subject
// to the terms of the Apache license (version 2.0), a copy of which is
// provided alongside this project.

package vcard

import (
	"io"
	"strings"
	"unicode/utf8"
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
func (r *UnfoldingReader) Read(bs []byte) (n int, err error) {
	for i := range bs {
		b, err := r.ReadByte()
		if err != nil {
			return i, err
		}
		bs[i] = b
	}
	return len(bs), nil
}

// ReadByte reads a single byte from the reader.
func (r *UnfoldingReader) ReadByte() (byte, error) {
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
			b3, err := r.readByte()
			if err != nil {
				return '\n', nil
			}
			if b3 == ' ' || b3 == '\t' {
				return r.ReadByte()
			}
			r.unread = append(r.unread, b3)
			return '\n', nil
		}
		r.unread = append(r.unread, b2)
	} else if b == '\n' {
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
	if b := r.peeked; b != -1 {
		r.peeked = -1
		return byte(b), nil
	}
	if len(r.unread) > 0 {
		b := r.unread[len(r.unread)-1]
		r.unread = r.unread[:len(r.unread)-1]
		return b, nil
	}

	var bs [1]byte
	n, err := r.r.Read(bs[:])
	if n == 0 {
		return 0, err
	}
	if bs[0] == '\n' {
		r.line++
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

// Fold folds a string, ensuring that no line exceeds the given number of bytes.
// It also converts simple '\n' line endings to "\r\n". The vCard specification
// recommends that output lines be folded to a width of at most 75 bytes,
// excluding the line ending.
//
// This implementation respects UTF-8, so it will never break the line in the
// middle of a codepoint. Also, note that if you have lines beginning with
// spaces (such as "hello\n world"), folding such a string and then unfolding
// it will not return the original string, since the space remains at the
// beginning of the next line.
func Fold(s string, width int) string {
	sb := new(strings.Builder)
	line := new(strings.Builder)
	// The maximum length of a line is width + 2 bytes, so we can
	// pre-allocate this for efficiency.
	line.Grow(width + 2)
	lastCR := false // whether the last character was '\r'

	for _, r := range s {
		if lastCR {
			if r == '\n' {
				sb.WriteString(line.String() + "\r\n")
				line.Reset()
				lastCR = false
				continue
			}
			if line.Len()+1 > width-2 {
				sb.WriteString(line.String() + "\r\n")
				line.Reset()
				line.WriteRune(' ')
			}
			line.WriteRune('\r')
			lastCR = false
		}
		if r == '\r' {
			lastCR = true
		} else if r == '\n' {
			sb.WriteString(line.String() + "\r\n")
			line.Reset()
		} else {
			if line.Len()+utf8.RuneLen(r) > width-2 {
				sb.WriteString(line.String() + "\r\n")
				line.Reset()
				line.WriteRune(' ')
			}
			line.WriteRune(r)
		}
	}
	if lastCR {
		if line.Len()+1 > width-2 {
			sb.WriteString(line.String() + "\r\n")
			line.Reset()
			line.WriteRune(' ')
		}
		line.WriteRune('\r')
	}
	sb.WriteString(line.String())
	return sb.String()
}
