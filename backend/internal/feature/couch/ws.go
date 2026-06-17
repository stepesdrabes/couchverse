package couch

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"couchverse/internal/httpx"
)

// WS upgrades a participant's connection. Authentication is by the couch cookie
// (set at join); the share token in the path is only routing. coder/websocket
// enforces same-origin by default (matching the auth CSRF posture).
func (h *Handlers) WS(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CouchCookie)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "no_couch_session", "join the couch session first")
		return
	}
	rm, pid, ok := h.hub.lookup(c.Value)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no_couch_session", "join the couch session first")
		return
	}
	isHost := rm.isHostParticipant(pid)

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the handshake error
	}
	h.hub.serveConn(ws, rm, pid, isHost)
}

// serveConn runs one connection's read/write pumps until it closes, then detaches
// it from the room. A watcher cancels the shared context as soon as either pump
// begins closing, unblocking the other.
func (h *Hub) serveConn(ws *websocket.Conn, rm *room, pid string, isHost bool) {
	defer ws.CloseNow()

	c := &conn{
		ws:     ws,
		room:   rm,
		pid:    pid,
		isHost: isHost,
		send:   make(chan []byte, sendBuffer),
		closed: make(chan struct{}),
	}
	if !rm.attach(c) {
		ws.Close(websocket.StatusGoingAway, "session ended")
		return
	}
	defer rm.detach(c)

	ctx, cancel := context.WithCancel(h.appCtx)
	defer cancel()
	go func() {
		select {
		case <-c.closed:
			cancel()
		case <-ctx.Done():
		}
	}()

	c.enqueue(rm.helloFrame(pid, isHost))

	var wg sync.WaitGroup
	wg.Add(2)
	go c.writePump(ctx, &wg)
	go c.readPump(ctx, &wg)
	wg.Wait()
}
