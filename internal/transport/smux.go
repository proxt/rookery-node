package transport

import (
	"fmt"
	"io"

	"github.com/xtaci/smux"
)

// smuxConfig tunes flow-control buffers well above smux's defaults.
// DefaultConfig's 64KB per-stream window caps a single stream's throughput
// to roughly window/RTT — at typical internet RTTs that's a few hundred
// KB/s at best, regardless of how fast the underlying link actually is.
func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.MaxStreamBuffer = 4 * 1024 * 1024   // per-stream window
	cfg.MaxReceiveBuffer = 64 * 1024 * 1024 // session-wide cap across all streams
	return cfg
}

// NewSmuxClient brings up an smux session as the stream-opening side.
func NewSmuxClient(conn io.ReadWriteCloser) (*smux.Session, error) {
	sess, err := smux.Client(conn, smuxConfig())
	if err != nil {
		return nil, fmt.Errorf("transport: smux client: %w", err)
	}
	return sess, nil
}

// NewSmuxServer brings up an smux session as the stream-accepting side.
func NewSmuxServer(conn io.ReadWriteCloser) (*smux.Session, error) {
	sess, err := smux.Server(conn, smuxConfig())
	if err != nil {
		return nil, fmt.Errorf("transport: smux server: %w", err)
	}
	return sess, nil
}
