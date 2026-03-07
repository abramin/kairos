// Package mcpserver implements a Model Context Protocol (MCP) server for Kairos.
// It exposes Kairos work items and projects as MCP tools over stdio/JSON-RPC 2.0,
// enabling Claude Code to read due-date data and bridge it to external calendar services.
package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const protocolVersion = "2024-11-05"

type jsonRPCRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *jsonRPCError    `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server is the Kairos MCP server. It reads JSON-RPC requests from stdin
// and writes responses to stdout, conforming to MCP protocol version 2024-11-05.
type Server struct {
	deps   Deps
	tools  []toolDef
	stdin  io.Reader
	stdout io.Writer
}

// NewServer creates a Server wired to the given repositories and os.Stdin/Stdout.
func NewServer(deps Deps) *Server {
	s := &Server{
		deps:   deps,
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}
	s.registerTools()
	return s
}

// Serve blocks, reading newline-delimited JSON-RPC messages from stdin until EOF.
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "parse error")
			continue
		}
		// Notifications have no ID — MCP spec says do not respond to them.
		if req.ID == nil {
			continue
		}
		s.dispatch(req)
	}
	return scanner.Err()
}

func (s *Server) dispatch(req jsonRPCRequest) {
	ctx := context.Background()
	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "kairos", "version": "1.0.0"},
		})
	case "tools/list":
		s.writeResult(req.ID, map[string]any{"tools": s.tools})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(req.ID, -32602, "invalid params")
			return
		}
		text, err := s.callTool(ctx, params.Name, params.Arguments)
		if err != nil {
			s.writeError(req.ID, -32000, err.Error())
			return
		}
		s.writeResult(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		})
	default:
		s.writeError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) writeResult(id *json.RawMessage, result any) {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.stdout, "%s\n", data)
}

func (s *Server) writeError(id *json.RawMessage, code int, msg string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: msg},
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.stdout, "%s\n", data)
}
