// Package bridge implements the CLI half of the K-055 native-messaging dual-host
// (GH#866): the `skrptiq` binary itself is the browser bridge, in two hidden
// modes.
//
//   - MCP mode (engine-spawned stdio child): serves the frozen tool catalogue to
//     the engine over stdio MCP AND owns the 0600 rendezvous socket cli.sock.
//   - Host mode (Chrome-spawned via the com.skrptiq.cli_bridge native-messaging
//     manifest): a stateless relay translating Chrome's length-prefixed frames
//     ⇄ newline-delimited JSON on the rendezvous socket.
//
// Chain: extension ⇄ (native messaging) ⇄ host-mode ⇄ cli.sock ⇄ MCP-mode ⇄ engine.
//
// The wire contract is protocol v2, FROZEN and owned by skrptiq-extension
// (shared/protocol.ts). This file mirrors it exactly; protocol_conformance_test.go
// guards the mirror against a mechanically-regenerated golden. Trust boundary =
// the pinned extension id (allowed_origins) + the 0600 socket. No port, no token.
package bridge

import "encoding/json"

// Protocol-v2 constants, mirrored from skrptiq-extension shared/protocol.ts and
// pinned by protocol_conformance_test.go against protocol.golden.json.
const (
	// ProtocolVersion is the frozen wire version; an announce with any other v is
	// rejected loud (unsupported_protocol_version).
	ProtocolVersion = 2

	// DefaultInvokeTimeoutMS bounds one invoke; no result within it ⇒ tool_timeout.
	DefaultInvokeTimeoutMS = 30_000

	// NMHostToExtMaxBytes is Chrome's 1 MB cap on the host→extension direction
	// (commands are tiny; we fail loud rather than let Chrome drop an over-cap frame).
	NMHostToExtMaxBytes = 1024 * 1024
	// NMExtToHostMaxBytes is Chrome's 4 GB cap on the extension→host direction
	// (results/snapshots ride here, effectively unconstrained).
	NMExtToHostMaxBytes = 4 * 1024 * 1024 * 1024
)

// BridgeErrorCode is the frozen failure taxonomy. Every failure surfaces as one
// of these and is mapped to an MCP ToolResult{isError:true} so the engine always
// records a FAILED step — never an empty success.
type BridgeErrorCode string

const (
	ErrUnsupportedProtocolVersion BridgeErrorCode = "unsupported_protocol_version"
	ErrUnknownTool                BridgeErrorCode = "unknown_tool"
	ErrBadRequest                 BridgeErrorCode = "bad_request"
	ErrInternalError              BridgeErrorCode = "internal_error"
	ErrAppNotRunning              BridgeErrorCode = "app_not_running"
	ErrHostUnavailable            BridgeErrorCode = "host_unavailable"
	ErrNativeDisconnected         BridgeErrorCode = "native_disconnected"
	ErrExtensionDisconnected      BridgeErrorCode = "extension_disconnected"
	ErrOriginNotGranted           BridgeErrorCode = "origin_not_granted"
	ErrCapabilityNotGranted       BridgeErrorCode = "capability_not_granted"
	ErrTabNotFound                BridgeErrorCode = "tab_not_found"
	ErrSelectorNotFound           BridgeErrorCode = "selector_not_found"
	ErrAmbiguousTarget            BridgeErrorCode = "ambiguous_target"
	ErrToolTimeout                BridgeErrorCode = "tool_timeout"
)

// allErrorCodes is the Go-side set, asserted equal to the golden in the
// conformance test (drift guard).
var allErrorCodes = []BridgeErrorCode{
	ErrUnsupportedProtocolVersion, ErrUnknownTool, ErrBadRequest, ErrInternalError,
	ErrAppNotRunning, ErrHostUnavailable, ErrNativeDisconnected, ErrExtensionDisconnected,
	ErrOriginNotGranted, ErrCapabilityNotGranted, ErrTabNotFound, ErrSelectorNotFound,
	ErrAmbiguousTarget, ErrToolTimeout,
}

// BridgeError is the structured failure carried on result/error messages.
type BridgeError struct {
	Code    BridgeErrorCode `json:"code"`
	Message string          `json:"message"`
	Detail  interface{}     `json:"detail,omitempty"`
}

// AnnouncedTool is one tool the live extension declares it can serve.
type AnnouncedTool struct {
	Name string `json:"name"`
}

// clientInfo / serverInfo identify each end on announce/ready.
type peerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// --- Extension → Bridge (over the rendezvous socket, relayed by the host) ---

// AnnounceMessage is the first frame the extension sends once its native port is
// up (replaces v1 hello; native messaging + allowed_origins are the auth).
type AnnounceMessage struct {
	Type   string          `json:"type"` // "announce"
	V      int             `json:"v"`
	Client peerInfo        `json:"client"`
	Tools  []AnnouncedTool `json:"tools"`
}

// ResultMessage is the reply to a prior invoke, correlated by ID.
type ResultMessage struct {
	Type    string       `json:"type"` // "result"
	ID      string       `json:"id"`
	OK      bool         `json:"ok"`
	Content interface{}  `json:"content,omitempty"`
	Error   *BridgeError `json:"error,omitempty"`
}

// --- Bridge → Extension (relayed by the host to the native port) ---

// ReadyMessage is the one-shot handshake ack on a valid announce.
type ReadyMessage struct {
	Type   string   `json:"type"` // "ready"
	V      int      `json:"v"`
	Server peerInfo `json:"server"`
}

// InvokeMessage is a tool call from the engine; the extension replies with result.
type InvokeMessage struct {
	Type string                 `json:"type"` // "invoke"
	ID   string                 `json:"id"`
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

// ErrorMessage is a terminal error surfaced to the extension (bad announce, or a
// host-synthesized app_not_running/native_disconnected). Never a silent no-op.
type ErrorMessage struct {
	Type    string          `json:"type"` // "error"
	Code    BridgeErrorCode `json:"code"`
	Message string          `json:"message"`
}

// messageType peeks the discriminator of a raw wire line without full decode.
func messageType(raw []byte) string {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.Type
}
