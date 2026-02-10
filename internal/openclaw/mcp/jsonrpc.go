package mcp

import (
	"encoding/json"
	"fmt"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func rpcErr(id json.RawMessage, code int, msg string, data any) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg, Data: data},
	}
}

func rpcOK(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func parseRPCRequest(body []byte) (rpcRequest, error) {
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return rpcRequest{}, err
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return rpcRequest{}, fmt.Errorf("unsupported jsonrpc version")
	}
	if req.Method == "" {
		return rpcRequest{}, fmt.Errorf("missing method")
	}
	return req, nil
}

