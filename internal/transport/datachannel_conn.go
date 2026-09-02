package transport

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/pion/webrtc/v4"
)

// ErrClosed is returned by Read/Write after the connection has been closed.
var ErrClosed = errors.New("transport: connection closed")

// readChBuffer bounds how many inbound messages OnMessage can queue ahead of
// Read() before it blocks. OnMessage runs on pion's own SCTP delivery
// goroutine, so blocking there stalls receipt for every stream multiplexed
// over this one DataChannel, not just the slow one — the receive-side
// equivalent of the bufferedAmount backpressure Write already gets, sized
// generously (smux's own per-stream window is 4MB) so a momentarily slow
// consumer downstream doesn't choke unrelated streams.
const readChBuffer = 2048

// DataChannelConn adapts a single WebRTC DataChannel into an
// io.ReadWriteCloser. Write applies bufferedAmount-based backpressure so a
// fast sender cannot grow the DataChannel's outbound queue without bound:
// once BufferedAmount exceeds highWaterMark, Write blocks until the
// low-threshold callback fires.
type DataChannelConn struct {
	dc *webrtc.DataChannel

	highWaterMark uint64
	lowSignal     chan struct{}

	readCh  chan []byte
	readBuf []byte

	closeOnce sync.Once
	closeCh   chan struct{}
}

// NewDataChannelConn wraps dc. lowThresholdBytes is passed to
// SetBufferedAmountLowThreshold; highWaterMarkBytes is the bufferedAmount a
// Write call blocks above until the low-threshold signal fires. dc must not
// yet be open; the caller is responsible for waiting on dc.OnOpen.
func NewDataChannelConn(dc *webrtc.DataChannel, lowThresholdBytes, highWaterMarkBytes uint64) *DataChannelConn {
	c := &DataChannelConn{
		dc:            dc,
		highWaterMark: highWaterMarkBytes,
		lowSignal:     make(chan struct{}, 1),
		readCh:        make(chan []byte, readChBuffer),
		closeCh:       make(chan struct{}),
	}

	dc.SetBufferedAmountLowThreshold(lowThresholdBytes)
	dc.OnBufferedAmountLow(func() {
		select {
		case c.lowSignal <- struct{}{}:
		default:
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		select {
		case c.readCh <- msg.Data:
		case <-c.closeCh:
		}
	})

	dc.OnClose(func() {
		c.closeOnce.Do(func() { close(c.closeCh) })
	})

	return c
}

// Read implements io.Reader.
func (c *DataChannelConn) Read(p []byte) (int, error) {
	if len(c.readBuf) == 0 {
		select {
		case buf := <-c.readCh:
			c.readBuf = buf
		case <-c.closeCh:
			return 0, io.EOF
		}
	}

	n := copy(p, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return n, nil
}

// Write implements io.Writer, blocking while the DataChannel's
// bufferedAmount is above highWaterMark.
func (c *DataChannelConn) Write(p []byte) (int, error) {
	if c.dc.BufferedAmount() > c.highWaterMark {
		select {
		case <-c.lowSignal:
		case <-c.closeCh:
			return 0, ErrClosed
		}
	}

	if err := c.dc.Send(p); err != nil {
		return 0, fmt.Errorf("transport: datachannel send: %w", err)
	}
	return len(p), nil
}

// Close implements io.Closer.
func (c *DataChannelConn) Close() error {
	c.closeOnce.Do(func() { close(c.closeCh) })
	return c.dc.Close()
}
