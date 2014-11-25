// package waddell implements a low-latency signaling server that allows peers
// to exchange small messages (up to around 64kB) over TCP.  It is named after
// William B. Waddell, one of the founders of the Pony Express.
//
// Peers are identified by randomly assigned peer ids (type 4 UUIDs), which are
// used to address messages to the peers.  For the scheme to work, peers must
// have some out-of-band mechanism by which they can exchange peer ids.  Note
// that as soon as one peer contacts another via waddell, the 2nd peer will have
// the 1st peer's address and be able to reply using it.
//
// Peers can obtain new ids simply by reconnecting to waddell, and depending on
// security requirements it may be a good idea to do so periodically.
//
//
// Here is an example exchange between two peers:
//
//   peer 1 -> waddell server : connect
//
//   waddell server -> peer 1 : send newly assigned peer id
//
//   peer 2 -> waddell server : connect
//
//   waddell server -> peer 2 : send newly assigned peer id
//
//   (out of band)            : peer 1 lets peer 2 know about its id
//
//   peer 2 -> waddell server : send message to peer 1
//
//   waddell server -> peer 1 : deliver message from peer 2 (includes peer 2's id)
//
//   peer 1 -> waddell server : send message to peer 2
//
//   etc ..
//
//
// Message structure on the wire (bits):
//
//   0-15    Frame Length    - waddell uses github.com/getlantern/framed to
//                             frame messages. framed uses the first 16 bits of
//                             the message to indicate the length of the frame
//                             (Little Endian).
//
//   16-79   Address Part 1  - 64-bit integer in Little Endian byte order for
//                             first half of peer id identifying recipient (on
//                             messages to waddell) or sender (on messages from
//                             waddell).
//
//   80-143  Address Part 2  - 64-bit integer in Little Endian byte order for
//                             second half of peer id
//
//   144+    Message Body    - whatever data the client sent
//
package waddell

import (
	"github.com/getlantern/buuid"
	"github.com/getlantern/golog"
)

const (
	PEER_ID_LENGTH   = buuid.EncodedLength
	WADDELL_OVERHEAD = 18 // bytes of overhead imposed by waddell
)

var (
	log = golog.LoggerFor("waddell.client")

	keepAlive = []byte{'k'}
)

// PeerId is an identifier for a waddell peer
type PeerId buuid.ID

// PeerIdFromString constructs a PeerId from the string-encoded version of a
// uuid.UUID.
func PeerIdFromString(s string) (PeerId, error) {
	id, err := buuid.FromString(s)
	return PeerId(id), err
}

func (id PeerId) String() string {
	return buuid.ID(id).String()
}

func readPeerId(b []byte) (PeerId, error) {
	id, err := buuid.Read(b)
	return PeerId(id), err
}

func randomPeerId() PeerId {
	return PeerId(buuid.Random())
}

func (id PeerId) write(b []byte) error {
	return buuid.ID(id).Write(b)
}

func (id PeerId) toBytes() []byte {
	return buuid.ID(id).ToBytes()
}
