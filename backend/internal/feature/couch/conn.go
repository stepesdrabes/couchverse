package couch

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const sendBuffer = 32

// conn is one WebSocket. A participant may own several (multi-tab). Exactly one
// goroutine writes (writePump, incl. pings) and one reads (readPump); coder's
// Conn supports that split without an extra mutex.
type conn struct {
	ws        *websocket.Conn
	room      *room
	pid       string
	isHost    bool
	send      chan []byte
	closed    chan struct{}
	once      sync.Once
	emojiLast time.Time // readPump-only: cheap per-conn emoji throttle
}

// enqueue queues a frame without ever blocking the fan-out. A client too slow to
// drain its buffer is dropped; it reconnects and resyncs from the next hello.
func (c *conn) enqueue(frame []byte) {
	select {
	case c.send <- frame:
	default:
		c.beginClose()
	}
}

func (c *conn) beginClose() {
	c.once.Do(func() { close(c.closed) })
}

// drainAndClose flushes any already-queued frames (e.g. session_ended) then
// closes the socket. Uses a fresh context since the conn context is cancelled.
func (c *conn) drainAndClose() {
	for {
		select {
		case frame := <-c.send:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = c.ws.Write(ctx, websocket.MessageText, frame)
			cancel()
		default:
			c.ws.Close(websocket.StatusNormalClosure, "")
			return
		}
	}
}

func (c *conn) writePump(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.beginClose()
	defer recoverLog("writePump")

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			c.drainAndClose()
			return
		case <-c.closed:
			c.drainAndClose()
			return
		case frame := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Write(wctx, websocket.MessageText, frame)
			cancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Ping(pctx) // a dead peer fails to pong -> teardown
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (c *conn) readPump(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.beginClose()
	defer recoverLog("readPump")

	c.ws.SetReadLimit(16 * 1024)
	for {
		var env Envelope
		// No per-read deadline: followers are mostly silent. Liveness comes from
		// the write pump's pings; ctx cancellation unblocks this read on teardown.
		if err := wsjson.Read(ctx, c.ws, &env); err != nil {
			return
		}
		c.room.onClientMessage(c, env)
	}
}

func recoverLog(where string) {
	if r := recover(); r != nil {
		slog.Error("couch goroutine panic", "where", where, "recover", r)
	}
}
