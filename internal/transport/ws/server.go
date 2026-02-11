package ws

import (
	"crypto/rand"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/multiworld"
	"voxelcraft.ai/internal/sim/world"
)

type Server struct {
	world   *world.World
	manager *multiworld.Manager
	log     *log.Logger

	upgrader websocket.Upgrader
}

func NewServer(w *world.World, logger *log.Logger) *Server {
	s := &Server{
		world: w,
		log:   logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  64 * 1024,
			WriteBufferSize: 64 * 1024,
			CheckOrigin:     func(r *http.Request) bool { return true }, // dev default
		},
	}
	return s
}

func NewManagedServer(m *multiworld.Manager, logger *log.Logger) *Server {
	s := &Server{
		manager: m,
		log:     logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  64 * 1024,
			WriteBufferSize: 64 * 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
	return s
}

type connSession struct {
	AgentID string
	WorldID string
	Out     chan []byte
	Delta   bool
	ProtocolVersion string
	SessionID       string
	mu              sync.Mutex
	acksByActID     map[string]protocol.AckMsg
}

func (s *Server) Handler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		sess, ok := s.handshake(conn)
		if !ok {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Writer goroutine.
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case b, ok := <-sess.Out:
					if !ok {
						return
					}
					b = adaptOutboundProtocolVersion(b, sess.ProtocolVersion)
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
						cancel()
						return
					}
				}
			}
		}()

		// Reader loop.
		for {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				cancel()
				break
			}
			base, err := protocol.DecodeBase(msg)
			if err != nil {
				continue
			}
			if base.Type != protocol.TypeAct {
				continue
			}
			var act protocol.ActMsg
			if err := json.Unmarshal(msg, &act); err != nil {
				if sess.ProtocolVersion == "1.1" {
					sendAckToOut(sess.Out, protocol.AckMsg{
						Type:            protocol.TypeAck,
						ProtocolVersion: sess.ProtocolVersion,
						AckFor:          act.ActID,
						Accepted:        false,
						Code:            "E_PROTO_BAD_REQUEST",
						Message:         "bad ACT payload",
						ServerTick:      s.currentTick(sess.WorldID),
						WorldID:         sess.WorldID,
					})
				}
				continue
			}
			if !protocol.IsSupportedVersion(act.ProtocolVersion) {
				if sess.ProtocolVersion == "1.1" {
					sendAckToOut(sess.Out, protocol.AckMsg{
						Type:            protocol.TypeAck,
						ProtocolVersion: sess.ProtocolVersion,
						AckFor:          act.ActID,
						Accepted:        false,
						Code:            "E_PROTO_BAD_REQUEST",
						Message:         "unsupported protocol_version",
						ServerTick:      s.currentTick(sess.WorldID),
						WorldID:         sess.WorldID,
					})
				}
				continue
			}
			if sess.ProtocolVersion == "1.1" {
				if strings.TrimSpace(act.ActID) == "" {
					sendAckToOut(sess.Out, protocol.AckMsg{
						Type:            protocol.TypeAck,
						ProtocolVersion: sess.ProtocolVersion,
						AckFor:          "",
						Accepted:        false,
						Code:            "E_PROTO_BAD_REQUEST",
						Message:         "missing act_id",
						ServerTick:      s.currentTick(sess.WorldID),
						WorldID:         sess.WorldID,
					})
					continue
				}
				if strings.TrimSpace(act.BasedOnObsID) == "" {
					sendAckToOut(sess.Out, protocol.AckMsg{
						Type:            protocol.TypeAck,
						ProtocolVersion: sess.ProtocolVersion,
						AckFor:          act.ActID,
						Accepted:        false,
						Code:            "E_PROTO_BAD_REQUEST",
						Message:         "missing based_on_obs_id",
						ServerTick:      s.currentTick(sess.WorldID),
						WorldID:         sess.WorldID,
					})
					continue
				}
				if isMutatingAct(act) && strings.TrimSpace(act.ExpectedWorldID) == "" {
					sendAckToOut(sess.Out, protocol.AckMsg{
						Type:            protocol.TypeAck,
						ProtocolVersion: sess.ProtocolVersion,
						AckFor:          act.ActID,
						Accepted:        false,
						Code:            "E_PROTO_BAD_REQUEST",
						Message:         "expected_world_id required for mutating ACT",
						ServerTick:      s.currentTick(sess.WorldID),
						WorldID:         sess.WorldID,
					})
					continue
				}
				if cached, ok := sess.lookupAck(act.ActID); ok {
					sendAckToOut(sess.Out, cached)
					continue
				}
				ack := protocol.AckMsg{
					Type:            protocol.TypeAck,
					ProtocolVersion: sess.ProtocolVersion,
					AckFor:          act.ActID,
					Accepted:        true,
					ServerTick:      s.currentTick(sess.WorldID),
					WorldID:         sess.WorldID,
				}
				sess.rememberAck(act.ActID, ack)
				sendAckToOut(sess.Out, ack)
			}
			if s.manager != nil {
				nextWorld, _ := s.manager.RouteAct(ctx, &multiworld.Session{
					AgentID:      sess.AgentID,
					CurrentWorld: sess.WorldID,
					DeltaVoxels:  sess.Delta,
					Out:          sess.Out,
				}, act)
				if nextWorld != "" {
					sess.WorldID = nextWorld
				}
				continue
			}
			s.world.Inbox() <- world.ActionEnvelope{AgentID: sess.AgentID, Act: act}
		}

		// Cleanup.
		if s.manager != nil {
			s.manager.Leave(multiworld.Session{
				AgentID:      sess.AgentID,
				CurrentWorld: sess.WorldID,
				DeltaVoxels:  sess.Delta,
				Out:          sess.Out,
			})
		} else {
			s.world.Leave() <- sess.AgentID
		}
	}
}

func (s *Server) handshake(conn *websocket.Conn) (connSession, bool) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return connSession{}, false
	}

	base, err := protocol.DecodeBase(msg)
	if err != nil || base.Type != protocol.TypeHello {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "expected HELLO"), time.Now().Add(time.Second))
		return connSession{}, false
	}

	var hello protocol.HelloMsg
	if err := json.Unmarshal(msg, &hello); err != nil {
		return connSession{}, false
	}
	selectedVersion, ok := protocol.SelectVersion(hello.SupportedVersions, hello.ProtocolVersion)
	if !ok {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "bad protocol_version"), time.Now().Add(time.Second))
		return connSession{}, false
	}
	if hello.AgentName == "" {
		hello.AgentName = "agent"
	}

	maxQ := hello.Capabilities.MaxQueue
	if maxQ <= 0 {
		maxQ = 8
	}
	if maxQ > 64 {
		maxQ = 64
	}
	out := make(chan []byte, maxQ)

	// Optional: resume an existing agent (reconnect).
	resumeToken := ""
	if hello.Auth != nil {
		resumeToken = strings.TrimSpace(hello.Auth.Token)
	}

	if s.manager != nil {
		var (
			resp world.JoinResponse
			sess multiworld.Session
		)
		if resumeToken != "" {
			ss, rr, err := s.manager.Attach(resumeToken, hello.Capabilities.DeltaVoxels, out)
			if err == nil {
				sess = ss
				resp = rr
			}
		}
		if resp.Welcome.AgentID == "" {
			ss, rr, err := s.manager.Join(hello.AgentName, hello.Capabilities.DeltaVoxels, out, hello.WorldPreference)
		if err != nil {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "join failed"), time.Now().Add(time.Second))
			return connSession{}, false
		}
		sess = ss
		resp = rr
	}
	applyWelcomeVersion(&resp.Welcome, selectedVersion)
		if err := writeJSON(conn, resp.Welcome); err != nil {
			return connSession{}, false
		}
		for _, c := range resp.Catalogs {
			c.ProtocolVersion = selectedVersion
			if err := writeJSON(conn, c); err != nil {
				return connSession{}, false
			}
		}
		return connSession{
			AgentID: sess.AgentID,
			WorldID: sess.CurrentWorld,
			Out:     out,
			Delta:   hello.Capabilities.DeltaVoxels,
			ProtocolVersion: selectedVersion,
			SessionID:       resp.Welcome.SessionID,
			acksByActID:     map[string]protocol.AckMsg{},
		}, true
	}

	var resp world.JoinResponse
	if resumeToken != "" {
		respCh := make(chan world.JoinResponse, 1)
		s.world.Attach() <- world.AttachRequest{
			ResumeToken: resumeToken,
			DeltaVoxels: hello.Capabilities.DeltaVoxels,
			Out:         out,
			Resp:        respCh,
		}
		resp = <-respCh
	}
	if resp.Welcome.AgentID == "" {
		// Fresh join.
		respCh := make(chan world.JoinResponse, 1)
		s.world.Join() <- world.JoinRequest{
			Name:        hello.AgentName,
			DeltaVoxels: hello.Capabilities.DeltaVoxels,
			Out:         out,
			Resp:        respCh,
		}
		resp = <-respCh
	}

	// Send welcome + catalogs immediately.
	applyWelcomeVersion(&resp.Welcome, selectedVersion)
	if err := writeJSON(conn, resp.Welcome); err != nil {
		return connSession{}, false
	}
	for _, c := range resp.Catalogs {
		c.ProtocolVersion = selectedVersion
		if err := writeJSON(conn, c); err != nil {
			return connSession{}, false
		}
	}

	return connSession{
		AgentID: resp.Welcome.AgentID,
		WorldID: resp.Welcome.CurrentWorldID,
		Out:     out,
		Delta:   hello.Capabilities.DeltaVoxels,
		ProtocolVersion: selectedVersion,
		SessionID:       resp.Welcome.SessionID,
		acksByActID:     map[string]protocol.AckMsg{},
	}, true
}

func (s *Server) currentTick(worldID string) uint64 {
	if s.manager != nil {
		if rt := s.manager.Runtime(worldID); rt != nil && rt.World != nil {
			return rt.World.CurrentTick()
		}
		return 0
	}
	if s.world != nil {
		return s.world.CurrentTick()
	}
	return 0
}

func isMutatingAct(act protocol.ActMsg) bool {
	return len(act.Instants) > 0 || len(act.Tasks) > 0 || len(act.Cancel) > 0
}

func sendAckToOut(out chan []byte, ack protocol.AckMsg) {
	b, err := json.Marshal(ack)
	if err != nil {
		return
	}
	// ACK should not evict OBS payloads from the queue; try a bounded enqueue.
	select {
	case out <- b:
	case <-time.After(200 * time.Millisecond):
	}
}

func applyWelcomeVersion(w *protocol.WelcomeMsg, selectedVersion string) {
	if w == nil {
		return
	}
	w.ProtocolVersion = selectedVersion
	w.SelectedVersion = selectedVersion
	w.ServerCapabilities = protocol.ServerCapabilities{Ack: true, EventBatch: true, Idempotency: true}
	if w.SessionID == "" {
		w.SessionID = newSessionID()
	}
}

func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b[:])
}

func adaptOutboundProtocolVersion(raw []byte, selectedVersion string) []byte {
	if selectedVersion == "" || len(raw) == 0 {
		return raw
	}
	base, err := protocol.DecodeBase(raw)
	if err != nil {
		return raw
	}
	if base.ProtocolVersion == selectedVersion {
		return raw
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	m["protocol_version"] = selectedVersion
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return b
}

func (s *connSession) lookupAck(actID string) (protocol.AckMsg, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acksByActID == nil {
		s.acksByActID = map[string]protocol.AckMsg{}
	}
	ack, ok := s.acksByActID[actID]
	return ack, ok
}

func (s *connSession) rememberAck(actID string, ack protocol.AckMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acksByActID == nil {
		s.acksByActID = map[string]protocol.AckMsg{}
	}
	s.acksByActID[actID] = ack
	if len(s.acksByActID) > 2048 {
		// Keep map bounded (simple truncate strategy).
		s.acksByActID = map[string]protocol.AckMsg{actID: ack}
	}
}

func sendLatestBytes(ch chan []byte, b []byte) {
	select {
	case ch <- b:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- b:
	default:
	}
}

func writeJSON(conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		if errors.Is(err, websocket.ErrCloseSent) {
			return err
		}
		return err
	}
	return nil
}
