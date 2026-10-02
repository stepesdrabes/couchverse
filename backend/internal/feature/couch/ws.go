package couch

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"couchverse/internal/httpx"
)

// WS upgrades a participant's connection. Authentication is by the participant token,
// from the couch cookie (browsers) or the X-Couch-Token header (native clients); the share
// token in the path is only routing. coder/websocket enforces same-origin for browsers by
// default (matching the auth CSRF posture); native clients send no Origin.
func (h *Handlers) WS(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get(CouchTokenHeader)
	if c, err := r.Cookie(CouchCookie); err == nil && token == "" {
		token = c.Value
	}
	ref, ok := h.hub.lookup(token)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no_couch_session", "join the couch session first")
		return
	}
	isHost := !ref.remote && ref.room.isHostParticipant(ref.pid)

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the handshake error
	}
	h.hub.serveConn(ws, ref.room, ref.pid, isHost, ref.remote)
}

// serveConn runs one connection's read/write pumps until it closes, then detaches
// it from the room. A closing reader stops the writer through c.closed; the reader
// is stopped only once the writer is done, because cancelling a read closes the
// socket at once and would cut off the frames the writer still flushes
// (session_ended above all).
func (h *Hub) serveConn(ws *websocket.Conn, rm *room, pid string, isHost, remote bool) {
	defer ws.CloseNow()

	c := &conn{
		ws:     ws,
		room:   rm,
		pid:    pid,
		isHost: isHost,
		remote: remote,
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

	c.enqueue(rm.helloFrame(c))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer cancel()
		c.writePump(ctx, &wg)
	}()
	go c.readPump(ctx, &wg)
	wg.Wait()
}
