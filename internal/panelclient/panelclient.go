// Package panelclient is a node's connection to its panel: it verifies
// client session tokens locally (no network round-trip needed — the token
// is signed with this node's own API key), and periodically reports
// liveness and per-subscription traffic totals back to the panel.
package panelclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/proxt/rookery-node/internal/signaling"
)

// heartbeatInterval is how often the node pings the panel to say it's alive.
const heartbeatInterval = 30 * time.Second

// reportInterval is how often accumulated traffic counters are flushed to
// the panel.
const reportInterval = 30 * time.Second

// httpTimeout bounds a single heartbeat/report request.
const httpTimeout = 10 * time.Second

// Client is a node's handle to its panel registration.
type Client struct {
	panelAddr string
	nodeID    string
	apiKey    []byte

	httpClient *http.Client

	mu       sync.Mutex
	counters map[string]*counter // subscription ID -> accumulated bytes
}

type counter struct {
	up, down uint64
}

// New builds a Client for a node registered with the panel at panelAddr as
// nodeID, authenticated with apiKey (issued by the panel when the node was
// added).
func New(panelAddr, nodeID, apiKey string) *Client {
	return &Client{
		panelAddr:  strings.TrimSuffix(panelAddr, "/"),
		nodeID:     nodeID,
		apiKey:     []byte(apiKey),
		httpClient: &http.Client{Timeout: httpTimeout},
		counters:   make(map[string]*counter),
	}
}

// VerifyToken checks a client-presented session token against this node's
// own API key — no call to the panel is made. It also rejects a token that
// was scoped to a different node, in case of panel misconfiguration.
func (c *Client) VerifyToken(token string) (signaling.Claims, error) {
	claims, err := signaling.VerifyToken(c.apiKey, token, time.Now())
	if err != nil {
		return signaling.Claims{}, err
	}
	if claims.NodeID != c.nodeID {
		return signaling.Claims{}, fmt.Errorf("panelclient: token scoped to node %q, not this node", claims.NodeID)
	}
	return claims, nil
}

// AddBytes accumulates traffic for subscriptionID, to be flushed to the
// panel on the next report interval.
func (c *Client) AddBytes(subscriptionID string, up, down uint64) {
	if up == 0 && down == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cnt, ok := c.counters[subscriptionID]
	if !ok {
		cnt = &counter{}
		c.counters[subscriptionID] = cnt
	}
	cnt.up += up
	cnt.down += down
}

// Run sends periodic heartbeats and traffic reports until ctx is canceled.
func (c *Client) Run(ctx context.Context) {
	hbTicker := time.NewTicker(heartbeatInterval)
	defer hbTicker.Stop()
	reportTicker := time.NewTicker(reportInterval)
	defer reportTicker.Stop()

	c.heartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			c.report(context.Background())
			return
		case <-hbTicker.C:
			c.heartbeat(ctx)
		case <-reportTicker.C:
			c.report(ctx)
		}
	}
}

func (c *Client) heartbeat(ctx context.Context) {
	if err := c.post(ctx, "/api/nodes/heartbeat", struct{}{}); err != nil {
		slog.Warn("panelclient: heartbeat", "error", err)
	}
}

type reportEntry struct {
	SubscriptionID string `json:"subscription_id"`
	BytesUp        uint64 `json:"bytes_up"`
	BytesDown      uint64 `json:"bytes_down"`
}

func (c *Client) report(ctx context.Context) {
	c.mu.Lock()
	if len(c.counters) == 0 {
		c.mu.Unlock()
		return
	}
	entries := make([]reportEntry, 0, len(c.counters))
	for subID, cnt := range c.counters {
		entries = append(entries, reportEntry{SubscriptionID: subID, BytesUp: cnt.up, BytesDown: cnt.down})
	}
	c.counters = make(map[string]*counter)
	c.mu.Unlock()

	if err := c.post(ctx, "/api/nodes/report", entries); err != nil {
		slog.Warn("panelclient: report traffic", "error", err)
	}
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.panelAddr+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", c.nodeID)
	req.Header.Set("X-Node-Key", string(c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("panel returned status %d", resp.StatusCode)
	}
	return nil
}
