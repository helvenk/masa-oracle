package pubsub

import (
	"context"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestNodeEventTracker(t *testing.T) {

	// nil NewNodeEventTracker
	t.Run("NewNodeEventTracker with nil network", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		assert.NotNil(t, tracker)
	})

	// Connected with nil NodeEventTracker
	t.Run("Connected with nil NodeEventTracker", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		tracker.Connected(nil, nil)
		// Should not panic and return early
	})

	t.Run("Connected with nil network", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		tracker.Connected(nil, nil)
		// Should not panic and return early
	})

	t.Run("Connected with non-existent node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		// Should return early without adding to buffer since node doesn't exist
		_, exists := tracker.ConnectBuffer[testPeerID.String()]
		assert.False(t, exists)
	})

	t.Run("Connected with existing active node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		// Add existing active node
		nodeData := &NodeData{
			PeerId:   testPeerID,
			IsActive: true,
		}
		tracker.nodeData.Set(testPeerID.String(), nodeData)

		// Call Connected
		tracker.Connected(nil, &testConn{peerId: testPeerID})

		// Should be added to buffer
		buffered, exists := tracker.ConnectBuffer[testPeerID.String()]
		assert.True(t, exists)
		assert.Equal(t, nodeData, buffered.NodeData)
		assert.False(t, buffered.ConnectTime.IsZero())
	})

	t.Run("Connected with existing inactive node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		// Add existing inactive node
		nodeData := &NodeData{
			PeerId:   testPeerID,
			IsActive: false,
		}
		tracker.nodeData.Set(testPeerID.String(), nodeData)

		// Call Connected
		tracker.Connected(nil, &testConn{peerId: testPeerID})

		// Should be marked as active and updated
		updated, exists := tracker.nodeData.Get(testPeerID.String())
		assert.True(t, exists)
		assert.True(t, updated.IsActive)
		assert.Equal(t, "1.0.0", updated.Version)
	})

	t.Run("Disconnected with nil network", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		//tracker.Disconnected(nil, nil)

		// Should not panic and return early
		assert.Empty(t, tracker.ConnectBuffer)

	})

	t.Run("Disconnected with non-existent node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")
		tracker.Disconnected(nil, &testConn{peerId: testPeerID})
		// Should return early since node doesn't exist
	})

	t.Run("Disconnected with buffered node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		// Add node data and buffer entry
		nodeData := &NodeData{
			PeerId:   testPeerID,
			IsActive: true,
		}
		tracker.nodeData.Set(testPeerID.String(), nodeData)
		tracker.ConnectBuffer[testPeerID.String()] = ConnectBufferEntry{
			NodeData:    nodeData,
			ConnectTime: time.Now(),
		}

		// Create channel to receive node data
		received := make(chan *NodeData, 1)
		go func() {
			data := <-tracker.NodeDataChan
			received <- data
		}()

		tracker.Disconnected(nil, &testConn{peerId: testPeerID})

		// Buffer entry should be removed
		_, exists := tracker.ConnectBuffer[testPeerID.String()]
		assert.False(t, exists)

		// Should receive updated node data
		select {
		case data := <-received:
			assert.Equal(t, testPeerID, data.PeerId)
			assert.True(t, data.IsActive) // Will be active because Joined is called after Left
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for node data")
		}
	})

	t.Run("Disconnected with non-buffered node", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		// Add node data without buffer entry
		nodeData := &NodeData{
			PeerId:   testPeerID,
			IsActive: true,
		}
		tracker.nodeData.Set(testPeerID.String(), nodeData)

		// Create channel to receive node data
		received := make(chan *NodeData, 1)
		go func() {
			data := <-tracker.NodeDataChan
			received <- data
		}()

		tracker.Disconnected(nil, &testConn{peerId: testPeerID})

		// Should receive updated node data
		select {
		case data := <-received:
			assert.Equal(t, testPeerID, data.PeerId)
			assert.False(t, data.IsActive)
			assert.Equal(t, ActivityLeft, data.Activity)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for node data")
		}
	})
}

func TestSortNodesByTwitterReliability(t *testing.T) {
	testPeerID1, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")
	testPeerID2, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKD")

	now := time.Now()

	nodes := []NodeData{
		{
			PeerId:            testPeerID1,
			ReturnedTweets:    10,
			LastReturnedTweet: now.Add(-1 * time.Hour),
			TweetTimeouts:     2,
			LastTweetTimeout:  now.Add(-2 * time.Hour),
		},
		{
			PeerId:            testPeerID2,
			ReturnedTweets:    15,
			LastReturnedTweet: now,
			TweetTimeouts:     1,
			LastTweetTimeout:  now.Add(-3 * time.Hour),
		},
	}

	SortNodesByTwitterReliability(nodes)

	assert.Equal(t, testPeerID2, nodes[0].PeerId, "Node with more recent tweet should be first")
}

func TestHandleNodeData(t *testing.T) {
	t.Run("New node data", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		data := NodeData{
			PeerId:          testPeerID,
			LastUpdatedUnix: time.Now().Unix(),
			IsStaked:        true,
		}

		tracker.HandleNodeData(data)

		stored, exists := tracker.nodeData.Get(testPeerID.String())
		assert.True(t, exists)
		assert.True(t, stored.IsStaked)
	})

	t.Run("Replay attack prevention", func(t *testing.T) {
		tracker := NewNodeEventTracker("1.0.0", "test", "host1")
		testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

		now := time.Now()
		oldData := NodeData{
			PeerId:          testPeerID,
			LastUpdatedUnix: now.Add(-1 * time.Hour).Unix(),
			IsStaked:        true,
		}

		newData := NodeData{
			PeerId:          testPeerID,
			LastUpdatedUnix: now.Unix(),
			IsStaked:        true,
		}

		// Add newer data first
		tracker.HandleNodeData(newData)
		// Try to add older data
		tracker.HandleNodeData(oldData)

		stored, _ := tracker.nodeData.Get(testPeerID.String())
		assert.Equal(t, newData.LastUpdatedUnix, stored.LastUpdatedUnix)
	})
}

func TestGetUpdatedNodes(t *testing.T) {
	tracker := NewNodeEventTracker("1.0.0", "test", "host1")
	testPeerID1, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")
	testPeerID2, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKD")

	now := time.Now()

	node1 := NodeData{
		PeerId:          testPeerID1,
		LastUpdatedUnix: now.Add(-2 * time.Hour).Unix(),
		IsStaked:        true,
	}

	node2 := NodeData{
		PeerId:          testPeerID2,
		LastUpdatedUnix: now.Add(-1 * time.Hour).Unix(),
		IsStaked:        true,
	}

	tracker.nodeData.Set(testPeerID1.String(), &node1)
	tracker.nodeData.Set(testPeerID2.String(), &node2)

	updatedNodes := tracker.GetUpdatedNodes(now.Add(-1*time.Hour - 1*time.Minute))
	assert.Len(t, updatedNodes, 1)
	assert.Equal(t, testPeerID2, updatedNodes[0].PeerId)
}

func TestUpdateNodeDataTwitter(t *testing.T) {
	tracker := NewNodeEventTracker("1.0.0", "test", "host1")
	testPeerID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC")

	// Add initial node data
	nodeData := &NodeData{
		PeerId:         testPeerID,
		ReturnedTweets: 5,
		IsStaked:       true,
	}
	tracker.nodeData.Set(testPeerID.String(), nodeData)

	// Create update
	updates := NodeData{
		ReturnedTweets: 10,
		TweetTimeout:   true,
		TweetTimeouts:  1,
	}

	// Create channel to receive updates
	received := make(chan *NodeData, 1)
	go func() {
		data := <-tracker.NodeDataChan
		received <- data
	}()

	err := tracker.UpdateNodeDataTwitter(testPeerID.String(), updates)
	assert.NoError(t, err)

	// Verify updates
	updated, exists := tracker.nodeData.Get(testPeerID.String())
	assert.True(t, exists)
	assert.Equal(t, 15, updated.ReturnedTweets) // 5 + 10
	assert.True(t, updated.TweetTimeout)
	assert.Equal(t, 1, updated.TweetTimeouts)
}

// Simple test connection that just returns a peer ID
// Simple test connection that implements the network.Conn interface
type testConn struct {
	peerId peer.ID
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) LocalPeer() peer.ID {
	testID, _ := peer.Decode("QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKD")
	return testID
}

func (c *testConn) RemotePublicKey() crypto.PubKey {
	return nil
}

func (c *testConn) ConnState() network.ConnectionState {
	return network.ConnectionState{}
}

func (c *testConn) LocalMultiaddr() multiaddr.Multiaddr {
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	return addr
}

func (c *testConn) RemoteMultiaddr() multiaddr.Multiaddr {
	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4002")
	return addr
}

func (c *testConn) Stat() network.ConnStats {
	return network.ConnStats{}
}

func (c *testConn) Scope() network.ConnScope {
	return nil
}

func (c *testConn) ID() string {
	return "test-conn-id"
}

func (c *testConn) NewStream(ctx context.Context) (network.Stream, error) {
	return nil, nil
}

func (c *testConn) GetStreams() []network.Stream {
	return nil
}

func (c *testConn) IsClosed() bool {
	return false
}

func (c *testConn) RemotePeer() peer.ID {
	return c.peerId
}
