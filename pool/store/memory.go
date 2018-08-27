package store

import (
	"sync"
	"time"
)

// MemoryStore implements an ephemeral in-memory store. It may not be a
// complete implementation but it's useful for testing.
func MemoryStore() *memoryStore {
	return &memoryStore{
		balances: map[Account]Balance{},
		nodes:    map[NodeID]Node{},
		nonces:   map[NodeID]int64{},
	}
}

// Assert Store implementation
var _ Store = &memoryStore{}

type memoryStore struct {
	mu sync.Mutex

	// Registered balances
	balances map[Account]Balance

	// Connected nodes
	nodes map[NodeID]Node

	nonces map[NodeID]int64
}

// CheckAndSaveNonce asserts that this is the highest nonce seen for this NodeID.
func (s *memoryStore) CheckAndSaveNonce(nodeID NodeID, nonce int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[nodeID] >= nonce {
		return ErrInvalidNonce
	}
	s.nonces[nodeID] = nonce
	return nil
}

// GetBalance returns the current balance for an account.
func (s *memoryStore) GetBalance(account Account) Balance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.balances[account]
}

// AddBalance adds some credit amount to that account balance.
func (s *memoryStore) AddBalance(account Account, credit Amount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.balances[account]
	b.Credit += credit
	if !ok {
		s.balances[account] = b
	}
	return nil
}

// SetNode adds a HostNode to the set of active host nodes.
func (s *memoryStore) SetNode(n Node, a Account) error {
	if n.ID == "" {
		return ErrMalformedNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if a != "" && n.balance == nil {
		b := s.balances[a]
		n.balance = &b
	}
	s.nodes[n.ID] = n
	return nil
}

// RemoveNode removes a HostNode.
func (s *memoryStore) RemoveNode(nodeID NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, nodeID)
	return nil
}

// ActiveHosts returns `limit`-number of `kind` nodes. This could be an
// empty list, if none are available.
func (s *memoryStore) ActiveHosts(kind string, limit int) []Node {
	seenSince := time.Now().Add(-2 * KeepaliveInterval)
	r := make([]Node, 0, limit)

	s.mu.Lock()
	// TODO: Do something other than random, such as by availability?
	for _, n := range s.nodes {
		// Ranging over a map is implicitly random, so
		// results are shuffled as is desireable.
		if !n.IsHost {
			continue
		}
		if kind != "" && n.Kind != kind {
			continue
		}
		if !n.LastSeen.After(seenSince) {
			continue
		}
		r = append(r, n)
		limit -= 1
		if limit == 0 {
			// If limit is originally 0, then limit is effectively ignored
			// since it will be <0.
			break
		}
	}
	s.mu.Unlock()
	return r
}

// UpdateNodePeers updates the Node.peers lookup with the current timestamp
// of nodes we know about. This is used as a keepalive, and to keep track of
// which client is connected to which host.
func (s *memoryStore) UpdateNodePeers(nodeID NodeID, peers []string) ([]Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, ErrUnregisteredNode
	}
	now := time.Now()
	node.LastSeen = now
	numUpdated := 0
	for _, peer := range peers {
		// Only update peers we already know about
		if _, ok := s.nodes[NodeID(peer)]; ok {
			node.peers[NodeID(peer)] = now
			numUpdated += 1
		}
	}

	if numUpdated == len(node.peers) {
		return nil, nil
	}
	inactive := []Node{}
	inactiveDeadline := now.Add(-2 * KeepaliveInterval)
	for nodeID, timestamp := range node.peers {
		if timestamp.Before(inactiveDeadline) {
			continue
		}
		delete(node.peers, nodeID)
		if node, ok := s.nodes[nodeID]; ok {
			inactive = append(inactive, node)
		}
	}

	return inactive, nil
}
