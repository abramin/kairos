package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates a Server with controlled stdin/stdout for testing.
func newTestServer(t *testing.T, input string) (*Server, *bytes.Buffer) {
	t.Helper()
	db := testutil.NewTestDB(t)
	deps := Deps{
		Projects:  repository.NewSQLiteProjectRepo(db),
		WorkItems: repository.NewSQLiteWorkItemRepo(db),
	}
	out := &bytes.Buffer{}
	s := &Server{
		deps:   deps,
		stdin:  strings.NewReader(input),
		stdout: out,
	}
	s.registerTools()
	return s, out
}

func parseResponse(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &m))
	return m
}

func firstLine(buf *bytes.Buffer) string {
	line, _ := buf.ReadString('\n')
	return strings.TrimRight(line, "\n")
}

func TestServer_Initialize(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())

	resp := parseResponse(t, firstLine(out))
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.NotNil(t, resp["result"])
	result := resp["result"].(map[string]any)
	assert.Equal(t, protocolVersion, result["protocolVersion"])
	assert.Nil(t, resp["error"])
}

func TestServer_ToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())

	resp := parseResponse(t, firstLine(out))
	assert.Nil(t, resp["error"])
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	assert.Len(t, tools, 3, "should register exactly 3 tools")

	names := make([]string, 0, 3)
	for _, tool := range tools {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	assert.Contains(t, names, "kairos_list_due_items")
	assert.Contains(t, names, "kairos_list_projects")
	assert.Contains(t, names, "kairos_get_project_items")
}

func TestServer_MethodNotFound(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":3,"method":"unknown/method","params":{}}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())

	resp := parseResponse(t, firstLine(out))
	assert.Nil(t, resp["result"])
	require.NotNil(t, resp["error"])
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, float64(-32601), errObj["code"])
}

func TestServer_ParseError(t *testing.T) {
	// Send malformed JSON — server should respond with parse error and continue.
	input := "not-valid-json\n" +
		`{"jsonrpc":"2.0","id":4,"method":"initialize","params":{}}` + "\n"
	s, out := newTestServer(t, input)
	require.NoError(t, s.Serve())

	// First line: parse error response (id=nil)
	line1 := firstLine(out)
	resp1 := parseResponse(t, line1)
	errObj := resp1["error"].(map[string]any)
	assert.Equal(t, float64(-32700), errObj["code"])

	// Second line: successful initialize response
	line2 := firstLine(out)
	resp2 := parseResponse(t, line2)
	assert.Nil(t, resp2["error"])
	assert.NotNil(t, resp2["result"])
}

func TestServer_ToolsCall_InvalidParams(t *testing.T) {
	// Malformed params JSON for tools/call
	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":"not-an-object"}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())

	resp := parseResponse(t, firstLine(out))
	require.NotNil(t, resp["error"])
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, float64(-32602), errObj["code"])
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"unknown_tool","arguments":{}}}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())

	resp := parseResponse(t, firstLine(out))
	require.NotNil(t, resp["error"])
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, float64(-32000), errObj["code"])
}

func TestServer_Notification_NoResponse(t *testing.T) {
	// A notification has no "id" — server should not respond.
	req := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	s, out := newTestServer(t, req+"\n")
	require.NoError(t, s.Serve())
	assert.Empty(t, out.String(), "notifications must not produce a response")
}

func TestServer_MultipleRequests_AllProcessed(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	s, out := newTestServer(t, input)
	require.NoError(t, s.Serve())

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	assert.Len(t, lines, 2, "should produce one response per request")
}
