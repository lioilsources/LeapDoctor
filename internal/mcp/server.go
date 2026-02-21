package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// ToolHandler dispatches tool calls by name.
type ToolHandler func(name string, args map[string]interface{}) ToolResult

// Server implements the MCP JSON-RPC 2.0 stdio transport.
type Server struct {
	scanner     *bufio.Scanner
	writer      *bufio.Writer
	tools       []Tool
	handler     ToolHandler
	instructions string
	dryRun      bool
}

func NewServer(in io.Reader, out io.Writer, tools []Tool, handler ToolHandler, instructions string, dryRun bool) *Server {
	scanner := bufio.NewScanner(in)
	// Increase buffer to 1MB (default 64KB is too small for large tool outputs)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	return &Server{
		scanner:      scanner,
		writer:       bufio.NewWriter(out),
		tools:        tools,
		handler:      handler,
		instructions: instructions,
		dryRun:       dryRun,
	}
}

func (s *Server) send(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
	s.writer.Flush()
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	s.send(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id interface{}, code int, msg string) {
	s.send(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	})
}

func (s *Server) handleCallTool(id interface{}, params json.RawMessage) {
	var p CallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.sendError(id, -32602, "invalid params: "+err.Error())
		return
	}

	result := s.handler(p.Name, p.Arguments)
	s.sendResult(id, result)
}

// Run starts the main JSON-RPC loop, reading from stdin and writing to stdout.
func (s *Server) Run() {
	log.Println("MCP server started (stdio transport)")

	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("parse error: %v", err)
			continue
		}

		// Notifications have no ID and don't require a response
		if req.ID == nil && req.Method != "" {
			if req.Method == "notifications/initialized" {
				log.Println("Client initialized")
			}
			continue
		}

		switch req.Method {
		case "initialize":
			instructions := s.instructions
			if s.dryRun {
				instructions += "\n\nDRY-RUN MODE ACTIVE: All destructive operations are simulated. No changes will be made to the system."
			}
			s.sendResult(req.ID, InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: Capabilities{
					Tools: &ToolsCapability{ListChanged: false},
				},
				ServerInfo: ServerInfo{
					Name:    "leapdoctor",
					Version: "1.0.0",
				},
				Instructions: instructions,
			})

		case "notifications/initialized":
			// Ignore

		case "tools/list":
			s.sendResult(req.ID, map[string]interface{}{
				"tools": s.tools,
			})

		case "tools/call":
			s.handleCallTool(req.ID, req.Params)

		case "ping":
			s.sendResult(req.ID, map[string]interface{}{})

		default:
			s.sendError(req.ID, -32601, "Method not found: "+req.Method)
		}
	}

	if err := s.scanner.Err(); err != nil {
		log.Printf("stdin error: %v", err)
	}
}
