package server

const (
	// protocolVersion is the latest MCP protocol version implemented by
	// this server. Returned during initialization per the MCP version
	// negotiation spec — the client decides whether it can work with
	// this version.
	protocolVersion = "2025-11-25"

	// serverName and serverVersion identify this implementation.
	serverName    = "mcpsmithy"
	serverVersion = "0.1.0"

	// MCP method names.
	methodInitialize             = "initialize"
	methodNotificationsInit      = "notifications/initialized"
	methodToolsList              = "tools/list"
	methodToolsCall              = "tools/call"
	methodPing                   = "ping"
	methodNotificationsCancelled = "notifications/cancelled"

	// Server-initiated notifications.
	methodToolsListChanged = "notifications/tools/list_changed"
)

// initializeParams is sent by the client in the "initialize" request.
type initializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      clientInfo `json:"clientInfo"`
}

// clientInfo describes the connecting client.
type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// initializeResult is returned by the server for "initialize".
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

// capabilities advertises what the server supports.
type capabilities struct {
	Tools map[string]any `json:"tools"`
}

// serverInfo describes this server.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toolDefinition describes a single tool exposed via tools/list.
type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// toolsListResult is the result of a tools/list call.
type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

// toolCallParams is sent by the client in a tools/call request.
type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// toolContent is a single content block inside a tool result.
type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolResult is the result payload for a tools/call response.
type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}
