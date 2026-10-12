package couchtv

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const sendBuffer = 32

// conn is one socket of a viewer. Exactly one goroutine writes (writePump, pings
// included) and one reads (readPump); coder/websocket supports that split without
// a mutex.
type conn struct {
	ws     *websocket.Conn
	viewer *viewer // set by attach, under the hub's lock
	send   chan []byte
	closed chan struct{}
	once   sync.Once
	last   *reported // readPump only
}

// reported is a socket's last report.
type reported struct {
	programDate time.Time
	at          time.Time
}

// serve runs a viewer's socket until it closes. A closing reader stops the writer
// through c.closed; the reader is stopped only once the writer is done, since
// cancelling a read closes the socket at once.
func (h *Hub) serve(ws *websocket.Conn, me CouchTvViewer, userID int64) {
	defer ws.CloseNow()
	c := &conn{ws: ws, send: make(chan []byte, sendBuffer), closed: make(chan struct{})}
	h.attach(c, me, userID)
	defer h.detach(c)

	ctx, cancel := context.WithCancel(h.ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer cancel()
		c.writePump(ctx, &wg)
	}()
	go h.readPump(ctx, c, &wg)
	wg.Wait()
}

// enqueue queues a frame without ever blocking the fan-out. A viewer too slow to
// drain their buffer is dropped; they reconnect and start again from hello.
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

func (c *conn) writePump(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.beginClose()
	defer recoverLog("writePump")

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			c.ws.Close(websocket.StatusGoingAway, "")
			return
		case <-c.closed:
			c.ws.Close(websocket.StatusNormalClosure, "")
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
			err := c.ws.Ping(pctx) // a dead peer fails to pong: teardown
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(ctx context.Context, c *conn, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.beginClose()
	defer recoverLog("readPump")

	c.ws.SetReadLimit(4 * 1024)
	for {
		var env Envelope
		// no read deadline: liveness comes from the write pump's pings, and the
		// context's end unblocks the read
		if err := wsjson.Read(ctx, c.ws, &env); err != nil {
			return
		}
		switch env.Type {
		case msgReaction:
			var cmd CouchTvReactionCommand
			if json.Unmarshal(env.Data, &cmd) == nil {
				h.react(c, cmd.Emoji)
			}
		case msgReport:
			var r CouchTvReport
			if json.Unmarshal(env.Data, &r) == nil {
				h.report(c, r.ProgramDate)
			}
		}
	}
}

func recoverLog(where string) {
	if r := recover(); r != nil {
		slog.Error("couchtv goroutine panic", "where", where, "recover", r)
	}
}
