package bytespool

import (
	"fmt"
	"io"
	"sync"
)

// ByteBuffer implements a simple byte buffer.
type ByteBuffer struct {
	// B is the underlying byte slice.
	B []byte
}

// Reset resets bb.
func (bb *ByteBuffer) Reset() {
	bb.B = bb.B[:0]
}

// Write appends p to bb.
func (bb *ByteBuffer) Write(p []byte) (int, error) {
	bb.B = append(bb.B, p...)
	return len(p), nil
}

// MustReadAt reads len(p) bytes starting from the given offset.
func (bb *ByteBuffer) MustReadAt(p []byte, offset int64) {
	if offset < 0 {
		panic(fmt.Sprintf("BUG: cannot read at negative offset=%d", offset))
	}
	if offset > int64(len(bb.B)) {
		panic(fmt.Sprintf("BUG: too big offset=%d; cannot exceed len(bb.B)=%d", offset, len(bb.B)))
	}
	if n := copy(p, bb.B[offset:]); n < len(p) {
		panic(fmt.Sprintf("BUG: EOF occurred after reading %d bytes out of %d bytes at offset %d", n, len(p), offset))
	}
}

// ReadFrom reads all the data from r to bb until EOF.
func (bb *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	b := bb.B
	bLen := len(b)
	b = resizeBytes(b, 4*1024)
	b = b[:cap(b)]
	offset := bLen
	for {
		if free := len(b) - offset; free < offset {
			n := len(b)
			b = append(b, make([]byte, n)...)
		}
		n, err := r.Read(b[offset:])
		offset += n
		if err != nil {
			bb.B = b[:offset]
			if err == io.EOF {
				err = nil
			}
			return int64(offset - bLen), err
		}
	}
}

// MustClose closes bb for subsequent re-use.
func (bb *ByteBuffer) MustClose() {
	// Do nothing, since certain code rely on bb reading after MustClose call.
}

// ByteBufferPool is a pool of ByteBuffers.
type ByteBufferPool struct {
	p sync.Pool
}

// Get obtains a ByteBuffer from bbp.
func (bbp *ByteBufferPool) Get() *ByteBuffer {
	bbv := bbp.p.Get()
	if bbv == nil {
		return &ByteBuffer{}
	}
	return bbv.(*ByteBuffer)
}

// Put puts bb into bbp.
func (bbp *ByteBufferPool) Put(bb *ByteBuffer) {
	bb.Reset()
	bbp.p.Put(bb)
}

// resizeBytes resizes b to n bytes and returns b (which may be newly allocated).
func resizeBytes(b []byte, n int) []byte {
	if nn := n - cap(b); nn > 0 {
		b = append(b[:cap(b)], make([]byte, nn)...)
	}
	return b[:n]
}
