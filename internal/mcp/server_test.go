package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func mockHandler(name string, args map[string]interface{}) ToolResult {
	return ToolOK("mock result for " + name)
}

func sendRequest(t *testing.T, method string, id int) Response {
	t.Helper()
	req := Request{JSONRPC: "2.0", ID: id, Method: method}
	data, _ := json.Marshal(req)

	in := bytes.NewBuffer(append(data, '\n'))
	out := &bytes.Buffer{}

	tools := []Tool{{Name: "test_tool", Description: "test"}}
	srv := NewServer(in, out, tools, mockHandler, "test instructions", false)
	srv.Run()

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, out.String())
	}
	return resp
}

func TestInitialize(t *testing.T) {
	resp := sendRequest(t, "initialize", 1)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(result), "2024-11-05") {
		t.Fatal("missing protocol version")
	}
	if !strings.Contains(string(result), "leapdoctor") {
		t.Fatal("missing server name")
	}
	if !strings.Contains(string(result), "test instructions") {
		t.Fatal("missing instructions")
	}
}

func TestPing(t *testing.T) {
	resp := sendRequest(t, "ping", 2)
	if resp.Error != nil {
		t.Fatalf("ping should succeed: %v", resp.Error)
	}
}

func TestUnknownMethod(t *testing.T) {
	resp := sendRequest(t, "nonexistent/method", 3)
	if resp.Error == nil {
		t.Fatal("should return error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestToolsList(t *testing.T) {
	resp := sendRequest(t, "tools/list", 4)
	if resp.Error != nil {
		t.Fatalf("tools/list should succeed: %v", resp.Error)
	}
	result, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(result), "test_tool") {
		t.Fatal("tools/list should include test_tool")
	}
}

func TestToolCall(t *testing.T) {
	params := CallToolParams{Name: "test_tool", Arguments: map[string]interface{}{"key": "val"}}
	paramsJSON, _ := json.Marshal(params)

	req := Request{JSONRPC: "2.0", ID: 5, Method: "tools/call", Params: paramsJSON}
	data, _ := json.Marshal(req)

	in := bytes.NewBuffer(append(data, '\n'))
	out := &bytes.Buffer{}

	tools := []Tool{{Name: "test_tool", Description: "test"}}
	srv := NewServer(in, out, tools, mockHandler, "", false)
	srv.Run()

	var resp Response
	json.Unmarshal(out.Bytes(), &resp)
	if resp.Error != nil {
		t.Fatalf("tool call should succeed: %v", resp.Error)
	}
	result, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(result), "mock result for test_tool") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestDryRunInstructions(t *testing.T) {
	req := Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	data, _ := json.Marshal(req)

	in := bytes.NewBuffer(append(data, '\n'))
	out := &bytes.Buffer{}

	srv := NewServer(in, out, nil, mockHandler, "base", true)
	srv.Run()

	var resp Response
	json.Unmarshal(out.Bytes(), &resp)
	result, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(result), "DRY-RUN MODE") {
		t.Fatal("dry-run mode should be mentioned in instructions")
	}
}

func TestNotificationIgnored(t *testing.T) {
	// Notification has no ID
	notif := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	in := bytes.NewBufferString(notif + "\n")
	out := &bytes.Buffer{}

	srv := NewServer(in, out, nil, mockHandler, "", false)
	srv.Run()

	if out.Len() > 0 {
		t.Fatalf("notification should not produce response, got: %s", out.String())
	}
}
