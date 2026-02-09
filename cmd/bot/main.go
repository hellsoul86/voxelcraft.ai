package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"

	"voxelcraft.ai/internal/protocol"
)

func main() {
	var (
		url  = flag.String("url", "ws://localhost:8080/v1/ws", "ws url")
		name = flag.String("name", "bot", "agent name")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[bot] ", log.LstdFlags|log.Lmicroseconds)
	conn, _, err := websocket.DefaultDialer.Dial(*url, nil)
	if err != nil {
		logger.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	hello := protocol.HelloMsg{
		Type:            protocol.TypeHello,
		ProtocolVersion: protocol.Version,
		AgentName:       *name,
		Capabilities: protocol.HelloCapabilities{
			DeltaVoxels: true,
			MaxQueue:    8,
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		logger.Fatalf("send HELLO: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	for {
		select {
		case <-stop:
			return
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		base, err := protocol.DecodeBase(msg)
		if err != nil {
			continue
		}
		switch base.Type {
		case protocol.TypeWelcome:
			var w protocol.WelcomeMsg
			if err := json.Unmarshal(msg, &w); err != nil {
				continue
			}
			logger.Printf("WELCOME agent_id=%s tick_rate=%d seed=%d", w.AgentID, w.WorldParams.TickRateHz, w.WorldParams.Seed)

		case protocol.TypeObs:
			var obs protocol.ObsMsg
			if err := json.Unmarshal(msg, &obs); err != nil {
				continue
			}
			handleObs(conn, logger, &obs)
		}
	}
}

func handleObs(conn *websocket.Conn, logger *log.Logger, obs *protocol.ObsMsg) {
	// Occasionally chat and move somewhere nearby.
	if obs.Tick%100 == 0 {
		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            obs.Tick,
			AgentID:         obs.AgentID,
			Instants: []protocol.InstantReq{
				{ID: fmt.Sprintf("I_say_%d", obs.Tick), Type: "SAY", Channel: "LOCAL", Text: fmt.Sprintf("tick=%d pos=%v", obs.Tick, obs.Self.Pos)},
			},
		}
		_ = conn.WriteJSON(act)
	}

	// Move every ~40 seconds.
	if obs.Tick%200 == 10 {
		r := rand.New(rand.NewSource(int64(obs.Tick) + time.Now().UnixNano()))
		tx := obs.Self.Pos[0] + r.Intn(15) - 7
		tz := obs.Self.Pos[2] + r.Intn(15) - 7
		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            obs.Tick,
			AgentID:         obs.AgentID,
			Tasks: []protocol.TaskReq{
				{ID: fmt.Sprintf("K_move_%d", obs.Tick), Type: "MOVE_TO", Target: [3]int{tx, obs.Self.Pos[1], tz}, Tolerance: 1.2},
			},
		}
		_ = conn.WriteJSON(act)
	}
}
