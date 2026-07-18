package bridge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// drive feeds newline-delimited JSON-RPC requests through serve() and returns the
// decoded responses keyed by id. The bridge has no connected extension, so
// tools/call fails loud with extension_disconnected — exactly the no-browser path.
func drive(t *testing.T, requests ...string) map[float64]rpcResponse {
	t.Helper()
	srv := newRendezvousServer("/tmp/unused-"+randomID()+".sock", nil)
	var out bytes.Buffer
	m, err := newMCPServer(srv, &out, nil)
	if err != nil {
		t.Fatalf("newMCPServer: %v", err)
	}
	if err := m.serve(strings.NewReader(strings.Join(requests, "\n") + "\n")); err != nil {
		t.Fatalf("serve: %v", err)
	}
	byID := map[float64]rpcResponse{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("bad response line %q: %v", line, err)
		}
		var id float64
		_ = json.Unmarshal(resp.ID, &id)
		byID[id] = resp
	}
	return byID
}

func TestMCP_Initialize(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	r, ok := resps[1]
	if !ok {
		t.Fatal("no initialize response")
	}
	var res map[string]any
	_ = json.Unmarshal(mustResult(t, r), &res)
	if res["protocolVersion"] != mcpProtocolVersion {
		t.Errorf("protocolVersion = %v, want %s", res["protocolVersion"], mcpProtocolVersion)
	}
}

func TestMCP_NotificationGetsNoResponse(t *testing.T) {
	// The initialized notification has no id and must not produce a response line.
	resps := drive(t,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`,
	)
	if _, ok := resps[0]; ok {
		t.Error("notification should not get a response")
	}
	if _, ok := resps[7]; !ok {
		t.Error("tools/list after a notification should still be answered")
	}
}

func TestMCP_ToolsList(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var res struct {
		Tools []struct {
			Name        string           `json:"name"`
			Annotations *ToolAnnotations `json:"annotations"`
		} `json:"tools"`
	}
	_ = json.Unmarshal(mustResult(t, resps[2]), &res)
	if len(res.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	var sawClick bool
	for _, tool := range res.Tools {
		if tool.Name == "click" {
			sawClick = true
			if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Error("click must expose destructiveHint:true on tools/list")
			}
		}
	}
	if !sawClick {
		t.Error("catalog missing click")
	}
}

func TestMCP_UnknownToolFailsLoud(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	var res toolResult
	_ = json.Unmarshal(mustResult(t, resps[3]), &res)
	if !res.IsError {
		t.Error("unknown tool must be isError")
	}
	if !strings.Contains(res.Content[0].Text, string(ErrUnknownTool)) {
		t.Errorf("expected unknown_tool code in %q", res.Content[0].Text)
	}
}

func TestMCP_ToolCallNoBrowserFailsLoud(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_page","arguments":{}}}`)
	var res toolResult
	_ = json.Unmarshal(mustResult(t, resps[4]), &res)
	if !res.IsError {
		t.Error("a catalog tool with no browser must fail loud (isError), never empty success")
	}
	if !strings.Contains(res.Content[0].Text, string(ErrExtensionDisconnected)) {
		t.Errorf("expected extension_disconnected in %q", res.Content[0].Text)
	}
}

func TestMCP_UnknownMethodErrors(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":5,"method":"does/not/exist"}`)
	if resps[5].Error == nil {
		t.Error("unknown method should return a JSON-RPC error")
	}
}

func mustResult(t *testing.T, r rpcResponse) json.RawMessage {
	t.Helper()
	if r.Error != nil {
		t.Fatalf("unexpected error response: %+v", r.Error)
	}
	b, _ := json.Marshal(r.Result)
	return b
}
