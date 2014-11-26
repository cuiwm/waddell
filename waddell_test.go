package waddell

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/getlantern/testify/assert"
)

const (
	Hello         = "Hello"
	HelloYourself = "Hello %s!"

	NumPeers = 100
)

// TestPeerIdRoundTrip makes sure that we can write and read a PeerId to/from a
// byte array.
func TestPeerIdRoundTrip(t *testing.T) {
	b := make([]byte, PeerIdLength)
	orig := randomPeerId()
	orig.write(b)
	read, err := readPeerId(b)
	if err != nil {
		t.Errorf("Unable to read peer id: %s", err)
	} else {
		if read != orig {
			t.Errorf("Read did not match original.  Expected: %s, Got: %s", orig, read)
		}
	}
}

func TestPeersPlainText(t *testing.T) {
	doTestPeers(t, false)
}

func TestPeersTLS(t *testing.T) {
	doTestPeers(t, true)
}

func doTestPeers(t *testing.T, useTLS bool) {
	pkfile := ""
	certfile := ""
	if useTLS {
		pkfile = "waddell_test_pk.pem"
		certfile = "waddell_test_cert.pem"
	}

	listener, err := Listen("localhost:0", pkfile, certfile)
	if err != nil {
		log.Fatalf("Unable to listen: %s", err)
	}

	go func() {
		server := &Server{}
		err = server.Serve(listener)
		if err != nil {
			log.Fatalf("Unable to start server: %s", err)
		}
	}()

	serverAddr := listener.Addr().String()

	dial := func() (net.Conn, error) {
		return net.Dial("tcp", serverAddr)
	}
	if useTLS {
		certBytes, err := ioutil.ReadFile(certfile)
		if err != nil {
			log.Fatalf("Unable to read cert from file: %s", err)
		}
		cert := string(certBytes)
		dial, err = Secured(dial, cert)
		if err != nil {
			log.Fatalf("Unable to secure dial function: %s", err)
		}
	}
	connect := func() *Client {
		client := &Client{
			Dial:              dial,
			ReconnectAttempts: 0,
		}
		err := client.Connect()
		if err != nil {
			log.Fatalf("Unable to connect client: %s", err)
		}
		return client
	}

	peersCh := make(chan *Client, NumPeers)
	// Connect clients
	for i := 0; i < NumPeers; i++ {
		go func() {
			peer := connect()
			assert.NoError(t, err, "Unable to connect peer")
			peersCh <- peer
		}()
	}

	peers := make([]*Client, 0, NumPeers)
	for i := 0; i < NumPeers; i++ {
		peers = append(peers, <-peersCh)
	}
	defer func() {
		for _, peer := range peers {
			peer.Close()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(NumPeers)

	// Send some large data to a peer that doesn't read, just to make sure we
	// handle blocked readers okay
	badPeer := connect()
	ld := largeData()
	for i := 0; i < 10; i++ {
		id, err := badPeer.ID()
		if err != nil {
			log.Fatalf("Unable to get peer id: %s", err)
		}
		badPeer.Send(id, ld)
	}

	// Simulate readers and writers
	for i := 0; i < NumPeers; i++ {
		peer := peers[i]
		isWriter := i%2 == 1

		if isWriter {
			go func() {
				// Simulate writer
				defer wg.Done()

				// Write to each reader
				for j := 0; j < NumPeers; j += 2 {
					recip := peers[j]
					recipId, err := recip.ID()
					if err != nil {
						log.Fatalf("Unable to get recip id: %s", err)
					}
					err = peer.Send(recipId, []byte(Hello))
					if err != nil {
						log.Fatalf("Unable to write hello: %s", err)
					} else {
						resp, err := peer.Receive()
						if err != nil {
							log.Fatalf("Unable to read response to hello: %s", err)
						} else {
							senderId, err := peer.ID()
							if err != nil {
								log.Fatalf("Unable to get sender id: %s", err)
							}
							assert.Equal(t, fmt.Sprintf(HelloYourself, senderId), string(resp.Body), "Response should match expected.")
							assert.Equal(t, recipId, resp.From, "Peer on response should match expected")
						}
					}
				}
			}()
		} else {
			go func() {
				// Simulate reader
				defer wg.Done()

				// Read from all readers
				for j := 1; j < NumPeers; j += 2 {
					err := peer.SendKeepAlive()
					if err != nil {
						log.Fatalf("Unable to send KeepAlive: %s", err)
					}
					msg, err := peer.Receive()
					if err != nil {
						log.Fatalf("Unable to read hello message: %s", err)
					}
					assert.Equal(t, Hello, string(msg.Body), "Hello message should match expected")
					err = peer.Send(msg.From, []byte(fmt.Sprintf(HelloYourself, msg.From)))
					if err != nil {
						log.Fatalf("Unable to write response to HELLO message: %s", err)
					}
				}
			}()
		}
	}

	wg.Wait()
}

// waitForServer waits for a TCP server to start at the given address, waiting
// up to the given limit and reporting an error to the given testing.T if the
// server didn't start within the time limit.
func waitForServer(addr string, limit time.Duration, t *testing.T) {
	cutoff := time.Now().Add(limit)
	for {
		if time.Now().After(cutoff) {
			t.Errorf("Server never came up at address %s", addr)
			return
		}
		c, err := net.DialTimeout("tcp", addr, limit)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func largeData() []byte {
	b := make([]byte, 60000)
	for i := 0; i < len(b); i++ {
		b[i] = byte(rand.Int())
	}
	return b
}
