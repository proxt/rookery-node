// Package session manages the lifecycle of one accepted client peer: waiting
// for its DataChannel to open, bringing up smux over it, and relaying every
// stream the client opens.
package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/proxt/rookery-node/internal/relay"
	"github.com/proxt/rookery-node/internal/transport"
)

// Config controls how an accepted peer session behaves.
type Config struct {
	MaxStreams                  int
	DialTimeout                 time.Duration
	AllowPrivateNet             bool
	BufferedAmountLowThreshold  uint64
	BufferedAmountHighWaterMark uint64
	DataChannelOpenTimeout      time.Duration
	// UserID identifies which user authenticated this
	// session (from its verified token), so relayed traffic can be
	// attributed to it.
	UserID string
	// OnBytes, if non-nil, is called as traffic is relayed for
	// UserID. See relay.Config.OnBytes.
	OnBytes func(userID string, up, down uint64)
}

// Run takes ownership of pc: it waits for the client's DataChannel to arrive
// on dcReady, brings up an smux server session over it, and relays every
// accepted stream until the peer disconnects or ctx is canceled. Run blocks
// until the session ends and always closes pc before returning.
func Run(ctx context.Context, pc *webrtc.PeerConnection, dcReady <-chan *webrtc.DataChannel, cfg Config) {
	defer pc.Close()

	var dc *webrtc.DataChannel
	select {
	case dc = <-dcReady:
	case <-time.After(cfg.DataChannelOpenTimeout):
		slog.Warn("session: datachannel did not arrive in time")
		return
	case <-ctx.Done():
		return
	}

	conn := transport.NewDataChannelConn(dc, cfg.BufferedAmountLowThreshold, cfg.BufferedAmountHighWaterMark)
	smuxSess, err := transport.NewSmuxServer(conn)
	if err != nil {
		slog.Error("session: bring up smux", "error", err)
		return
	}
	defer smuxSess.Close()

	slog.Info("session: established")
	defer slog.Info("session: ended")

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-smuxSess.CloseChan():
		case <-ctx.Done():
			smuxSess.Close()
		}
		cancel()
	}()

	relayCfg := relay.Config{DialTimeout: cfg.DialTimeout, AllowPrivateNet: cfg.AllowPrivateNet}
	if cfg.OnBytes != nil {
		relayCfg.OnBytes = func(up, down uint64) { cfg.OnBytes(cfg.UserID, up, down) }
	}
	sem := make(chan struct{}, cfg.MaxStreams)
	var wg sync.WaitGroup

	for {
		stream, err := smuxSess.AcceptStream()
		if err != nil {
			break
		}

		select {
		case sem <- struct{}{}:
		case <-sessionCtx.Done():
			stream.Close()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			relay.Handle(sessionCtx, stream, relayCfg)
		}()
	}

	wg.Wait()
}
