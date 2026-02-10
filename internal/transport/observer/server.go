package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/world"
)

type Server struct {
	world *world.World
	log   *log.Logger

	upgrader websocket.Upgrader
	nextID   atomic.Uint64
}

func NewServer(w *world.World, logger *log.Logger) *Server {
	return &Server{
		world: w,
		log:   logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  64 * 1024,
			WriteBufferSize: 64 * 1024,
			CheckOrigin:     func(r *http.Request) bool { return true }, // dev default
		},
	}
}

func (s *Server) BootstrapHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}

		cfg := s.world.Config()
		resp := observerproto.BootstrapResponse{
			ProtocolVersion: observerproto.Version,
			WorldID:         cfg.ID,
			Tick:            s.world.CurrentTick(),
			WorldParams: observerproto.WorldParams{
				TickRateHz: cfg.TickRateHz,
				ChunkSize:  [3]int{16, 16, cfg.Height},
				Height:     cfg.Height,
				Seed:       cfg.Seed,
				BoundaryR:  cfg.BoundaryR,
			},
			BlockPalette: s.world.BlockPalette(),
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(resp)
	}
}

func (s *Server) WSHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}

		conn, err := s.upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handshake: must send SUBSCRIBE first.
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var sub observerproto.SubscribeMsg
		if err := json.Unmarshal(msg, &sub); err != nil {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "bad subscribe"), time.Now().Add(time.Second))
			return
		}
		if sub.Type != "SUBSCRIBE" || sub.ProtocolVersion != observerproto.Version {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "expected SUBSCRIBE"), time.Now().Add(time.Second))
			return
		}

		normalizeSubscribe(&sub)

		sid := fmt.Sprintf("O%d", s.nextID.Add(1))
		tickOut := make(chan []byte, 8)
		dataOut := make(chan []byte, 4096)

		joinReq := world.ObserverJoinRequest{
			SessionID:   sid,
			TickOut:     tickOut,
			DataOut:     dataOut,
			ChunkRadius: sub.ChunkRadius,
			MaxChunks:   sub.MaxChunks,
		}
		select {
		case s.world.ObserverJoin() <- joinReq:
		default:
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "server busy"), time.Now().Add(time.Second))
			return
		}
		defer func() {
			select {
			case s.world.ObserverLeave() <- sid:
			default:
				// World loop is stopping; nothing else to do.
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Writer goroutine.
		writeErr := make(chan error, 1)
		go func() {
			for {
				select {
				case <-ctx.Done():
					writeErr <- ctx.Err()
					return
				case b, ok := <-dataOut:
					if !ok {
						writeErr <- nil
						return
					}
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
						writeErr <- err
						return
					}
				case b, ok := <-tickOut:
					if !ok {
						writeErr <- nil
						return
					}
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
						writeErr <- err
						return
					}
				}
			}
		}()

		// Reader loop: allow SUBSCRIBE updates.
		for {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var sub observerproto.SubscribeMsg
			if err := json.Unmarshal(msg, &sub); err != nil {
				continue
			}
			if sub.Type != "SUBSCRIBE" || sub.ProtocolVersion != observerproto.Version {
				continue
			}
			normalizeSubscribe(&sub)
			req := world.ObserverSubscribeRequest{
				SessionID:   sid,
				ChunkRadius: sub.ChunkRadius,
				MaxChunks:   sub.MaxChunks,
			}
			select {
			case s.world.ObserverSubscribe() <- req:
			default:
				// Drop updates under load; the client may resend.
			}
		}

		cancel()
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))

		// Best-effort wait for the writer to stop so it doesn't outlive conn.
		select {
		case <-writeErr:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func normalizeSubscribe(sub *observerproto.SubscribeMsg) {
	if sub.ChunkRadius <= 0 {
		sub.ChunkRadius = 6
	}
	if sub.ChunkRadius > 32 {
		sub.ChunkRadius = 32
	}
	if sub.MaxChunks <= 0 {
		sub.MaxChunks = 1024
	}
	if sub.MaxChunks > 16384 {
		sub.MaxChunks = 16384
	}
}

func isLoopbackRemote(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
