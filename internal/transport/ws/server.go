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
	"voxelcraft.ai/internal/sim/world"
)

type Server struct {
	world *world.World
	log   *log.Logger

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

func (s *Server) Handler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		agentID, out := s.handshake(conn)
		if agentID == "" {
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
				case b, ok := <-out:
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
			if act.ProtocolVersion != protocol.Version {
				continue
			}
			s.world.Inbox() <- world.ActionEnvelope{AgentID: agentID, Act: act}
		}

		// Cleanup.
		s.world.Leave() <- agentID
	}
}

func (s *Server) handshake(conn *websocket.Conn) (agentID string, out chan []byte) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return "", nil
	}

	base, err := protocol.DecodeBase(msg)
	if err != nil || base.Type != protocol.TypeHello {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "expected HELLO"), time.Now().Add(time.Second))
		return "", nil
	}

	var hello protocol.HelloMsg
	if err := json.Unmarshal(msg, &hello); err != nil {
		return "", nil
	}
	if hello.ProtocolVersion != protocol.Version {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "bad protocol_version"), time.Now().Add(time.Second))
		return "", nil
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
	out = make(chan []byte, maxQ)

	// Optional: resume an existing agent (reconnect).
	resumeToken := ""
	if hello.Auth != nil {
		resumeToken = strings.TrimSpace(hello.Auth.Token)
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
		return "", nil
	}
	for _, c := range resp.Catalogs {
		if err := writeJSON(conn, c); err != nil {
			return "", nil
		}
	}

	return resp.Welcome.AgentID, out
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
