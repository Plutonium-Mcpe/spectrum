package packet

const (
	IDConnectionRequest = iota + 500
	IDConnectionResponse
	IDFlush
	IDLatency
	IDTransfer
	IDUpdateCache
)

// SpectrumPM already speaks 506 (LOGIN) and 507 (DISCONNECT) on this wire, so the
// next free identifier is 508. Written out rather than carried by the iota above,
// so that inserting a packet there can never silently shift it onto theirs.
const IDDisconnectReason = 508
