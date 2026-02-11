package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
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
				continue
			}
			if !protocol.IsSupportedVersion(act.ProtocolVersion) {
				continue
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
	if !protocol.IsSupportedVersion(hello.ProtocolVersion) {
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
		if err := writeJSON(conn, resp.Welcome); err != nil {
			return connSession{}, false
		}
		for _, c := range resp.Catalogs {
			if err := writeJSON(conn, c); err != nil {
				return connSession{}, false
			}
		}
		return connSession{
			AgentID: sess.AgentID,
			WorldID: sess.CurrentWorld,
			Out:     out,
			Delta:   hello.Capabilities.DeltaVoxels,
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
	if err := writeJSON(conn, resp.Welcome); err != nil {
		return connSession{}, false
	}
	for _, c := range resp.Catalogs {
		if err := writeJSON(conn, c); err != nil {
			return connSession{}, false
		}
	}

	return connSession{
		AgentID: resp.Welcome.AgentID,
		WorldID: resp.Welcome.CurrentWorldID,
		Out:     out,
		Delta:   hello.Capabilities.DeltaVoxels,
	}, true
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
