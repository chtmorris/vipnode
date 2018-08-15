package pool

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// ErrNoHostNodes is returned when the pool does not have any hosts available.
var ErrNoHostNodes = errors.New("no host nodes available")

// ErrVerifyFailed is returned when a signature fails to verify. It embeds
// the underlying Cause.
type ErrVerifyFailed struct {
	Cause  error
	Method string
}

func (err ErrVerifyFailed) Error() string {
	return fmt.Sprintf("method %q failed to verify signature: %s", err.Method, err.Cause)
}

// New returns a VipnodePool implementation of Pool with the default memory
// store, which includes balance tracking.
func New() *VipnodePool {
	return &VipnodePool{
		// TODO: Replace with persistent store
		Store: store.MemoryStore(),
	}
}

// VipnodePool implements a Pool service with balance tracking.
type VipnodePool struct {
	Store store.Store
}

func (p *VipnodePool) verify(sig string, method string, nodeID string, nonce int64, args ...interface{}) error {
	if err := p.Store.CheckAndSaveNonce(store.NodeID(nodeID), nonce); err != nil {
		return ErrVerifyFailed{Cause: err, Method: method}
	}

	if err := request.Verify(sig, method, nodeID, nonce, args...); err != nil {
		return ErrVerifyFailed{Cause: err, Method: method}
	}
	return nil
}

// register associates a wallet account with a nodeID, and increments the account's credit.
func (p *VipnodePool) register(nodeID store.NodeID, account store.Account, credit store.Amount) error {
	// TODO: Check if nodeID is already registered to another balance, if so remove it
	return p.Store.AddBalance(account, credit)
}

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *VipnodePool) Update(ctx context.Context, sig string, nodeID string, nonce int64, peers []string) (*store.Balance, error) {
	if err := p.verify(sig, "vipnode_update", nodeID, nonce, peers); err != nil {
		return nil, err
	}

	return nil, errors.New("not implemented yet")
}

// Host registers a full node to participate as a vipnode host in this pool.
func (p *VipnodePool) Host(ctx context.Context, sig string, nodeID string, nonce int64, kind string, nodeURI string) error {
	if err := p.verify(sig, "vipnode_host", nodeID, nonce, kind); err != nil {
		return err
	}

	// Confirm that nodeURI matches nodeID
	uri, err := url.Parse(nodeURI)
	if err != nil {
		return err
	}

	if uri.Hostname() != nodeID {
		return fmt.Errorf("nodeID [%s...] does not match nodeURI: %s", nodeID[:8], nodeURI)
	}

	logger.Printf("New host: %s", nodeURI)

	return p.Store.SetHostNode(store.HostNode{
		ID:       store.NodeID(nodeID),
		URI:      nodeURI,
		Kind:     kind,
		LastSeen: time.Now(),
	})
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]store.HostNode, error) {
	// FIXME: Kind might be insufficient: We need to distinguish between full node vs parity LES and geth LES.
	// TODO: Send a whitelist request, only return subset of nodes that responded in time.
	if err := p.verify(sig, "vipnode_connect", nodeID, nonce, kind); err != nil {
		return nil, err
	}

	// TODO: Unhardcode 3?
	r := p.Store.GetHostNodes(kind, 3)
	if len(r) == 0 {
		return nil, ErrNoHostNodes
	}

	return r, nil
}

// Disconnect removes the node from the pool and stops accumulating respective balances.
func (p *VipnodePool) Disconnect(ctx context.Context, sig string, nodeID string, nonce int64) error {
	if err := p.verify(sig, "vipnode_disconnect", nodeID, nonce); err != nil {
		return err
	}

	// TODO: ...
	return nil
}

// Withdraw schedules a balance withdraw for a node
func (p *VipnodePool) Withdraw(ctx context.Context, sig string, nodeID string, nonce int64) error {
	if err := p.verify(sig, "vipnode_withdraw", nodeID, nonce); err != nil {
		return err
	}

	// TODO:
	return errors.New("not implemented yet")
}

// Ping returns "pong", used for testing.
func (p *VipnodePool) Ping(ctx context.Context) string {
	return "pong"
}
