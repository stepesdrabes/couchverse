package couch

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
)

// attach registers a connection. It bumps the owning participant's conn count
// and, if the host has reconnected, cancels the grace timer.
func (rm *room) attach(c *conn) bool {
	rm.mu.Lock()
	if !rm.live {
		rm.mu.Unlock()
		return false
	}
	rm.conns[c] = struct{}{}
	hostReturned := false
	// a remote is not the host being present: it plays nothing
	if p := rm.participants[c.pid]; p != nil && !c.remote {
		p.connCount++
		p.disconnectedAt = time.Time{}
		if p.IsHost && p.connCount == 1 {
			if rm.graceTimer != nil {
				rm.graceTimer.Stop()
				rm.graceTimer = nil
			}
			if rm.state.Away {
				rm.state.Away = false
				hostReturned = true
			}
		}
	}
	rm.lastActive = time.Now()
	rm.mu.Unlock()

	if hostReturned {
		rm.broadcastExceptHost(msgHostReturned, nil)
		rm.broadcastHostState()
	}
	return true
}

// detach removes a connection. When the host loses its last connection, the
// session enters the away/grace state.
func (rm *room) detach(c *conn) {
	rm.mu.Lock()
	delete(rm.conns, c)
	if rm.playing == c {
		rm.playing = nil
	}
	startGrace := false
	if p := rm.participants[c.pid]; p != nil && !c.remote {
		p.connCount--
		if p.connCount <= 0 {
			p.connCount = 0
			p.disconnectedAt = time.Now()
			if p.IsHost && rm.live {
				rm.state.Away = true
				startGrace = true
			}
		}
	}
	rm.mu.Unlock()

	if startGrace {
		rm.hub.startHostGrace(rm)
		rm.broadcastExceptHost(msgHostAway, CouchHostAway{GraceSeconds: int(rm.hub.hostGrace.Seconds())})
		rm.broadcastHostState()
	}
}

func (rm *room) snapshotConns() []*conn {
	conns := make([]*conn, 0, len(rm.conns))
	for c := range rm.conns {
		conns = append(conns, c)
	}
	return conns
}

func (rm *room) broadcast(typ string, data any) {
	frame := mustEnvelope(typ, data)
	rm.mu.Lock()
	conns := rm.snapshotConns()
	rm.mu.Unlock()
	for _, c := range conns {
		c.enqueue(frame)
	}
}

// broadcastExceptHost relays to followers only - the host is the source of truth
// and must not react to its own echoed state.
func (rm *room) broadcastExceptHost(typ string, data any) {
	frame := mustEnvelope(typ, data)
	rm.mu.Lock()
	conns := make([]*conn, 0, len(rm.conns))
	for c := range rm.conns {
		if !c.isHost {
			conns = append(conns, c)
		}
	}
	rm.mu.Unlock()
	for _, c := range conns {
		c.enqueue(frame)
	}
}

func (rm *room) broadcastHostState() {
	rm.mu.Lock()
	st := rm.state
	rm.mu.Unlock()
	rm.broadcastExceptHost(msgHostState, st)
}

func (rm *room) broadcastParticipants() {
	rm.mu.Lock()
	parts := rm.participantViewsLocked()
	conns := rm.snapshotConns()
	rm.mu.Unlock()
	frame := mustEnvelope(msgParticipants, CouchParticipants{Participants: parts})
	for _, c := range conns {
		c.enqueue(frame)
	}
}

func (rm *room) helloFrame(c *conn) []byte {
	role := roleFollower
	switch {
	case c.remote:
		role = roleRemote
	case c.isHost:
		role = roleHost
	}
	snap := rm.snapshotFor(c.pid, role)
	return mustEnvelope(msgHello, CouchHello{
		SessionID:       snap.SessionID,
		MyParticipantID: c.pid,
		Role:            role,
		State:           snap.State,
		Participants:    snap.Participants,
		ServerTimeMs:    rm.hub.nowMs(),
	})
}

// applyHostState stores a new authoritative state from the host, stamping the
// server timestamp + sequence. Returns whether the loaded media changed.
func (rm *room) applyHostState(cmd CouchHostStateCommand) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	switched := cmd.Media != rm.state.Media
	rm.state.Media = cmd.Media
	rm.state.Playing = cmd.Playing
	rm.state.PositionSeconds = cmd.PositionSeconds
	rm.state.ServerTimestampMs = rm.hub.nowMs()
	rm.state.Seq++
	rm.state.Away = false
	rm.lastActive = time.Now()
	return switched
}

func (rm *room) currentMedia() CouchMediaRef {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.state.Media
}

// recomputeAllowed refreshes the stream-authorization set and the watch-time
// title after a media switch.
func (rm *room) recomputeAllowed() {
	allowed, titleID, err := rm.hub.resolveAllowed(rm.hub.appCtx, rm.currentMedia())
	if err != nil {
		return
	}
	rm.mu.Lock()
	rm.allowedMediaIDs = allowed
	rm.titleID = titleID
	rm.mu.Unlock()
}

// onClientMessage routes an inbound frame. host_state is host-only; emoji is
// open to anyone but validated and throttled.
func (rm *room) onClientMessage(c *conn, env Envelope) {
	switch env.Type {
	case msgHostState:
		if !c.isHost {
			c.ws.Close(websocket.StatusPolicyViolation, "host only")
			c.beginClose()
			return
		}
		var cmd CouchHostStateCommand
		if json.Unmarshal(env.Data, &cmd) != nil {
			return
		}
		// the device that reports its state is the one playing, which remotes steer
		rm.mu.Lock()
		rm.playing = c
		rm.mu.Unlock()
		if rm.applyHostState(cmd) {
			rm.recomputeAllowed()
			rm.mu.Lock()
			data := CouchMediaChanged{Media: rm.state.Media, Seq: rm.state.Seq}
			rm.mu.Unlock()
			rm.broadcastExceptHost(msgMediaChanged, data)
		}
		rm.broadcastHostState()

	case msgEmoji:
		var cmd CouchEmojiCommand
		if json.Unmarshal(env.Data, &cmd) != nil {
			return
		}
		emoji := sanitizeEmoji(cmd.Emoji)
		if emoji == "" {
			return
		}
		if time.Since(c.emojiLast) < 350*time.Millisecond {
			return
		}
		c.emojiLast = time.Now()
		rm.mu.Lock()
		if p := rm.participants[c.pid]; p != nil {
			p.emojiCount++ // flushed to the reaction counter by the hub's accrual tick
		}
		rm.mu.Unlock()
		rm.broadcast(msgEmoji, CouchEmoji{FromParticipantID: c.pid, Emoji: emoji})

	case msgRemote:
		if !c.remote {
			c.ws.Close(websocket.StatusPolicyViolation, "remote only")
			c.beginClose()
			return
		}
		var cmd CouchRemoteCommand
		if json.Unmarshal(env.Data, &cmd) != nil || !validRemote(cmd) {
			return
		}
		rm.mu.Lock()
		target := rm.playing
		rm.lastActive = time.Now()
		rm.mu.Unlock()
		if target != nil {
			target.enqueue(mustEnvelope(msgRemote, cmd))
		}

	case msgPaused:
		var cmd CouchPausedCommand
		if json.Unmarshal(env.Data, &cmd) != nil {
			return
		}
		changed := false
		rm.mu.Lock()
		if p := rm.participants[c.pid]; p != nil && p.Paused != cmd.Paused {
			p.Paused = cmd.Paused
			changed = true
		}
		rm.mu.Unlock()
		if changed {
			rm.broadcastParticipants()
		}
	}
}

func validRemote(cmd CouchRemoteCommand) bool {
	switch cmd.Action {
	case "play", "pause", "next", "previous":
		return true
	case "seek":
		return cmd.PositionSeconds != nil && *cmd.PositionSeconds >= 0
	}
	return false
}

// sanitizeEmoji bounds a reaction to a short unicode string (the picker yields a
// single grapheme; we cannot allow-list the full set).
func sanitizeEmoji(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 32 || !utf8.ValidString(s) {
		return ""
	}
	if utf8.RuneCountInString(s) > 8 {
		return ""
	}
	return s
}
