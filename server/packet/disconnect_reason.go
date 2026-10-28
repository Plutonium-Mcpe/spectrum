package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// DisconnectReason is sent by the server just before it closes a session, to tell the proxy why.
//
// The proxy used to infer the reason from the text shown to the player, by comparing it against a
// handful of hardcoded English strings. Anything it did not recognise was treated as a restart: the
// disconnect was swallowed, the player bounced to another lobby, and a banned player ended up with a
// generic connection error instead of the reason for their ban. The code below travels beside the
// message so the proxy never has to guess, and a reworded message cannot silently change behaviour.
type DisconnectReason struct {
	// Reason is the machine-readable cause. See the ReasonXXX constants.
	Reason string
	// Message is the text to show the player. Empty means the server has nothing to add, and the
	// proxy falls back to whatever it would normally display for that reason.
	Message string
}

const (
	// ReasonBan — the player is banned. The proxy must show Message and must not bounce them.
	ReasonBan = "ban"
	// ReasonWhitelist — the player is not allowed on this server.
	ReasonWhitelist = "whitelist"
	// ReasonFull — the server has no room left.
	ReasonFull = "full"
	// ReasonKick — a moderator disconnected the player.
	ReasonKick = "kick"
	// ReasonRestart — the server is going down; bouncing the player elsewhere is the right call.
	ReasonRestart = "restart"
)

// ID ...
func (pk *DisconnectReason) ID() uint32 {
	return IDDisconnectReason
}

// Marshal ...
func (pk *DisconnectReason) Marshal(io protocol.IO) {
	io.String(&pk.Reason)
	io.String(&pk.Message)
}
