package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// The MCP server the engine connects to over stdio (MCP mode). The engine's
// client (skrptiq-app engine/mcp/transport_stdio.go) speaks LINE-DELIMITED
// JSON-RPC 2.0 — a request per line, a response per line — and negotiates
// protocol 2024-11-05. This is a minimal hand-rolled server (no MCP SDK in the
// single Go binary) implementing exactly the four methods that client uses:
// initialize, notifications/initialized, tools/list, tools/call.
//
// tools/list answers from the frozen CATALOG (even with no browser). tools/call
// routes to the rendezvous server and maps every failure to a ToolResult with
// isError:true, so the engine records a FAILED step — never an empty success.
//
// stdout is the JSON-RPC transport; ALL logging goes to stderr.

const mcpProtocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// mcpServer serves stdio JSON-RPC backed by the rendezvous bridge + catalogue.
type mcpServer struct {
	bridge    *rendezvousServer
	tools     []CatalogTool
	toolNames map[string]struct{}
	out       io.Writer
	logf      func(string, ...interface{})
}

func newMCPServer(bridge *rendezvousServer, out io.Writer, logf func(string, ...interface{})) (*mcpServer, error) {
	tools, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}
	return &mcpServer{
		bridge:    bridge,
		tools:     tools,
		toolNames: catalogToolNames(tools),
		out:       out,
		logf:      logf,
	}, nil
}

// serve reads JSON-RPC requests line by line from in until EOF, handling each.
func (m *mcpServer) serve(in io.Reader) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			m.logf("[mcp] dropping unparseable request: %v", err)
			continue
		}
		m.handle(req)
	}
	return scanner.Err()
}

func (m *mcpServer) handle(req rpcRequest) {
	// A request with no id is a notification — never answered.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		m.reply(req.ID, map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]string{"name": serverInfo.Name, "version": serverInfo.Version},
		})
	case "notifications/initialized":
		// no-op notification
	case "tools/list":
		m.reply(req.ID, m.listTools())
	case "tools/call":
		res := m.callTool(req.Params)
		m.reply(req.ID, res)
	default:
		if !isNotification {
			m.replyError(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

func (m *mcpServer) listTools() map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(m.tools))
	for _, t := range m.tools {
		entry := map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		}
		if t.Annotations != nil {
			entry["annotations"] = t.Annotations
		}
		out = append(out, entry)
	}
	return map[string]interface{}{"tools": out}
}

func (m *mcpServer) callTool(params json.RawMessage) toolResult {
	var p struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return failResult(ErrBadRequest, "could not parse tools/call params")
	}
	if _, ok := m.toolNames[p.Name]; !ok {
		return failResult(ErrUnknownTool, fmt.Sprintf("No such tool in the catalog: %q.", p.Name))
	}
	outcome := m.bridge.invoke(p.Name, p.Arguments)
	if outcome.ok {
		return okResult(outcome.content)
	}
	return failResult(outcome.err.Code, outcome.err.Message)
}

// okResult mirrors the companion: text passthrough for strings, pretty JSON else.
func okResult(content interface{}) toolResult {
	var text string
	if s, ok := content.(string); ok {
		text = s
	} else {
		b, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			return failResult(ErrInternalError, "could not encode tool result")
		}
		text = string(b)
	}
	return toolResult{Content: []toolContent{{Type: "text", Text: text}}}
}

// failResult is a loud isError result — "<code>: <message>" so the engine step
// fails with a legible reason, never an empty success.
func failResult(code BridgeErrorCode, message string) toolResult {
	return toolResult{
		Content: []toolContent{{Type: "text", Text: fmt.Sprintf("%s: %s", code, message)}},
		IsError: true,
	}
}

func (m *mcpServer) reply(id json.RawMessage, result interface{}) {
	if len(id) == 0 || string(id) == "null" {
		return // notification — no response
	}
	m.write(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (m *mcpServer) replyError(id json.RawMessage, code int, message string) {
	m.write(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (m *mcpServer) write(resp rpcResponse) {
	line, err := json.Marshal(resp)
	if err != nil {
		m.logf("[mcp] marshal response: %v", err)
		return
	}
	if _, err := m.out.Write(append(line, '\n')); err != nil {
		m.logf("[mcp] write response: %v", err)
	}
}
