package badger

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/vipnode/vipnode/pool/store"
)

// TODO: Set reasonable expiration values?

type peers map[store.NodeID]time.Time

// Open returns a store.Store implementation using Badger as the storage
// driver. The store should be (*badgerStore).Close()'d after use.
func Open(opts badger.Options) (*badgerStore, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &badgerStore{db: db}, nil
}

var _ store.Store = &badgerStore{}

type badgerStore struct {
	db *badger.DB
}

func (s *badgerStore) Close() error {
	return s.db.Close()
}

func (s *badgerStore) hasKey(txn *badger.Txn, key []byte) bool {
	_, err := txn.Get(key)
	return err == nil
}

func (s *badgerStore) getItem(txn *badger.Txn, key []byte, into interface{}) error {
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	val, err := item.Value()
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(val)).Decode(&into)
}

func (s *badgerStore) setItem(txn *badger.Txn, key []byte, val interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err
	}
	return txn.Set(key, buf.Bytes())
}

func (s *badgerStore) CheckAndSaveNonce(nodeID store.NodeID, nonce int64) error {
	key := []byte(fmt.Sprintf("vip:nonce:%s", nodeID))
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			val, err := item.Value()
			if err != nil {
				return err
			}
			var lastNonce int64
			if err := gob.NewDecoder(bytes.NewReader(val)).Decode(&lastNonce); err != nil {
				return err
			}
			if lastNonce >= nonce {
				return store.ErrInvalidNonce
			}
		} else if err != nil {
			return err
		}

		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(nonce); err != nil {
			return err
		}
		return txn.Set(key, buf.Bytes())
	})
}

func (s *badgerStore) GetBalance(nodeID store.NodeID) (store.Balance, error) {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	var account store.Account

	var r store.Balance
	err := s.db.View(func(txn *badger.Txn) error {
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := s.getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		if err := s.getItem(txn, balanceKey, &r); err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		return nil
	})

	return r, err
}

func (s *badgerStore) AddBalance(nodeID store.NodeID, credit store.Amount) error {
	accountKey := []byte(fmt.Sprintf("vip:account:%s", nodeID))
	return s.db.Update(func(txn *badger.Txn) error {
		var account store.Account
		balanceKey := []byte(fmt.Sprintf("vip:trial:%s", nodeID))
		if err := s.getItem(txn, accountKey, &account); err == badger.ErrKeyNotFound {
			// No spendable account, use the trial account
		} else if err == nil {
			balanceKey = []byte(fmt.Sprintf("vip:balance:%s", account))
		} else {
			return err
		}
		var balance store.Balance
		if err := s.getItem(txn, balanceKey, &balance); err == badger.ErrKeyNotFound {
			// No balance = empty balance
		} else if err != nil {
			return err
		}
		balance.Credit += credit

		return s.setItem(txn, balanceKey, balance)
	})
}

func (s *badgerStore) GetSpendable(account store.Account, nodeID store.NodeID) (store.Balance, error) {
	return store.Balance{}, errors.New("not implemented")
}

func (s *badgerStore) SetSpendable(account store.Account, nodeID store.NodeID) error {
	// TODO: Migrate trial account if exists
	return errors.New("not implemented")
}

func (s *badgerStore) ActiveHosts(kind string, limit int) []store.Node {
	panic("not implemented")
}

func (s *badgerStore) GetNode(nodeID store.NodeID) (*store.Node, error) {
	key := []byte(fmt.Sprintf("vip:node:%s", nodeID))
	var r store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		return s.getItem(txn, key, &r)
	})
	if err == badger.ErrKeyNotFound {
		return nil, store.ErrUnregisteredNode
	} else if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *badgerStore) SetNode(n store.Node) error {
	if n.ID == "" {
		return store.ErrMalformedNode
	}
	key := []byte(fmt.Sprintf("vip:node:%s", n.ID))
	return s.db.Update(func(txn *badger.Txn) error {
		return s.setItem(txn, key, n)
	})
}

func (s *badgerStore) NodePeers(nodeID store.NodeID) ([]store.Node, error) {
	key := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	var r []store.Node
	err := s.db.View(func(txn *badger.Txn) error {
		return s.getItem(txn, key, &r)
	})
	if err == badger.ErrKeyNotFound {
		return nil, store.ErrUnregisteredNode
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *badgerStore) UpdateNodePeers(nodeID store.NodeID, peers []string) (inactive []store.NodeID, err error) {
	nodeKey := []byte(fmt.Sprintf("vip:node:%s", nodeID))
	peersKey := []byte(fmt.Sprintf("vip:peers:%s", nodeID))
	now := time.Now()
	var node store.Node
	var nodePeers map[store.NodeID]time.Time
	err = s.db.Update(func(txn *badger.Txn) error {
		// Update this node's LastSeen
		if err := s.getItem(txn, nodeKey, &node); err != nil {
			return err
		}
		node.LastSeen = now
		if err := s.setItem(txn, nodeKey, node); err != nil {
			return err
		}

		// Update peers
		if err := s.getItem(txn, peersKey, &nodePeers); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		numUpdated := 0
		for _, peerID := range peers {
			// Only update peers we already know about
			if s.hasKey(txn, []byte(fmt.Sprintf("vip:node:%s", peerID))) {
				nodePeers[store.NodeID(peerID)] = now
				numUpdated += 1
			}
		}

		if numUpdated == len(nodePeers) {
			return s.setItem(txn, peersKey, nodePeers)
		}

		inactiveDeadline := now.Add(-store.ExpireInterval)
		for nodeID, timestamp := range nodePeers {
			if timestamp.Before(inactiveDeadline) {
				continue
			}
			delete(nodePeers, nodeID)
			inactive = append(inactive, nodeID)
		}
		return s.setItem(txn, peersKey, nodePeers)
	})
	return
}
