package transport

import (
	"fmt"
	"io"

	"github.com/xtaci/smux"
)

// smuxConfig tunes flow-control buffers well above smux's defaults, and —
// critically — switches to protocol version 2.
//
// Version 1 (smux's default) has no real per-stream flow control at all:
// the receiver just stops reading off the wire once a single session-wide
// token bucket (MaxReceiveBuffer) is exhausted. A lone high-throughput
// stream never gets a per-stream window that scales with the link, so
// raising MaxStreamBuffer alone under v1 does nothing for it — confirmed
// against the vendored smux source (github.com/xtaci/smux, tryReadV1/
// writeV1 never reference peerWindow at all, that's a v2-only mechanism).
// Version 2 adds a real sliding window per stream, advertised as
// MaxStreamBuffer once the first window update round-trips — that's what
// actually lets one stream's throughput approach window/RTT instead of
// being capped by whatever the *session's* shared bucket happens to have
// left.
//
// Both sides of a session must agree on Version — smux doesn't negotiate
// it, each peer just assumes its own config. A v2 sender talking to a v1
// receiver stalls once it exhausts the 256KB initial window, since a v1
// peer never sends the window-update frames that would replenish it. The
// node and client must therefore ship this change together.
func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.MaxStreamBuffer = 16 * 1024 * 1024   // per-stream window once warmed up
	cfg.MaxReceiveBuffer = 128 * 1024 * 1024 // session-wide cap across all streams
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
