package acp

// The ACP methods and the keys and values wardcase's two ends share.
const (
	MethodInitialize        = "initialize"
	MethodSessionNew        = "session/new"
	MethodSessionPrompt     = "session/prompt"
	MethodSessionCancel     = "session/cancel"
	MethodSessionUpdate     = "session/update"
	MethodRequestPermission = "session/request_permission"

	// KeySessionID, KeyProtocolVersion and KeyStopReason are the JSON keys of the
	// params and results.
	KeySessionID       = "sessionId"
	KeyProtocolVersion = "protocolVersion"
	KeyStopReason      = "stopReason"
	KeyType            = "type"
	KeyText            = "text"

	// StopCancelled and StopEndTurn are the stop reasons wardcase meets.
	StopCancelled = "cancelled"
	StopEndTurn   = "end_turn"

	jsonrpcVersion = "2.0"
	outcomeKey     = "outcome"
)

// unsupported is the error for a method wardcase does not serve.
func unsupported(method string) *RPCError {
	return &RPCError{
		Code: codeMethodNotFound, Message: "unsupported " + method, Data: nil,
	}
}
