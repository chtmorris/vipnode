package pool

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vipnode/vipnode/internal/pretty"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

type hostService struct {
	store.Node
	jsonrpc2.Service
}

// New returns a VipnodePool implementation of Pool with the default memory
// store, which includes balance tracking.
func New() *VipnodePool {
	// TODO: Replace with persistent store
	storeDriver := store.MemoryStore()
	balanceManager := &payPerInterval{
		Store:             storeDriver,
		Interval:          time.Minute * 1,
		CreditPerInterval: 1000,
	}
	return &VipnodePool{
		Store:          storeDriver,
		BalanceManager: balanceManager,
		remoteHosts:    map[store.NodeID]jsonrpc2.Service{},
	}
}

const poolWhitelistTimeout = 5 * time.Second

// VipnodePool implements a Pool service with balance tracking.
type VipnodePool struct {
	Store          store.Store
	BalanceManager BalanceManager
	skipWhitelist  bool

	mu          sync.Mutex
	remoteHosts map[store.NodeID]jsonrpc2.Service
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

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *VipnodePool) Update(ctx context.Context, sig string, nodeID string, nonce int64, peers []string) (*UpdateResponse, error) {
	if err := p.verify(sig, "vipnode_update", nodeID, nonce, peers); err != nil {
		return nil, err
	}

	node, err := p.Store.GetNode(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}
	nodeBeforeUpdate := *node

	inactive, err := p.Store.UpdateNodePeers(store.NodeID(nodeID), peers)
	if err != nil {
		return nil, err
	}

	resp := UpdateResponse{
		InvalidPeers: make([]string, 0, len(inactive)),
	}
	for _, peer := range inactive {
		resp.InvalidPeers = append(resp.InvalidPeers, string(peer.ID))
	}
	validPeers, err := p.Store.NodePeers(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}

	// FIXME: Is there a bug here when a host is connected to another host?
	// TODO: Test InvalidPeers

	balance, err := p.BalanceManager.OnUpdate(nodeBeforeUpdate, validPeers)
	if err != nil {
		return nil, err
	}
	resp.Balance = &balance

	if node.IsHost {
		logger.Printf("Host update %q: %d peers, %d active, %d invalid. Balance: %d", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), balance.Credit)
	} else {
		logger.Printf("Client update %q: %d peers, %d active, %d invalid: Balance: %d", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), balance.Credit)

	}

	return &resp, nil
}

// Host registers a full node to participate as a vipnode host in this pool.
func (p *VipnodePool) Host(ctx context.Context, sig string, nodeID string, nonce int64, kind string, payout string, nodeURI string) error {
	if err := p.verify(sig, "vipnode_host", nodeID, nonce, kind, payout, nodeURI); err != nil {
		return err
	}

	// Confirm that nodeURI matches nodeID
	uri, err := url.Parse(nodeURI)
	if err != nil {
		return err
	}

	if uri.User.Username() != nodeID {
		return fmt.Errorf("nodeID %q does not match nodeURI: %s", pretty.Abbrev(nodeID), nodeURI)
	}

	// XXX: Confirm that it's a full node, not a light node.
	// XXX: Check versions

	logger.Printf("New %q host: %q", kind, nodeURI)

	node := store.Node{
		ID:       store.NodeID(nodeID),
		URI:      nodeURI,
		Kind:     kind,
		LastSeen: time.Now(),
		IsHost:   true,
	}
	err = p.Store.SetNode(node, store.Account(payout))
	if err != nil {
		return err
	}

	service, err := jsonrpc2.CtxService(ctx)
	if err != nil {
		return err
	}
	// FIXME: Clean up disconnected hosts
	p.mu.Lock()
	p.remoteHosts[node.ID] = service
	p.mu.Unlock()

	return nil
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]store.Node, error) {
	// FIXME: Should this be Client and vipnode_client?
	// FIXME: Kind might be insufficient: We need to distinguish between full node vs parity LES and geth LES.
	if err := p.verify(sig, "vipnode_connect", nodeID, nonce, kind); err != nil {
		return nil, err
	}

	// TODO: Unhardcode these
	numRequestHosts := 3

	r := p.Store.ActiveHosts(kind, numRequestHosts)
	if len(r) == 0 {
		logger.Printf("New %q client: %q (no active hosts found)", kind, pretty.Abbrev(nodeID))
		return nil, ErrNoHostNodes{}
	}

	if p.skipWhitelist {
		logger.Printf("New %q client: %q (%d hosts found, skipping whitelist)", kind, pretty.Abbrev(nodeID), len(r))
		return r, nil
	}

	errors := []error{}
	remotes := make([]hostService, 0, len(r))
	p.mu.Lock()
	for _, node := range r {
		remote, ok := p.remoteHosts[node.ID]
		if ok {
			remotes = append(remotes, hostService{
				node, remote,
			})
		} else {
			errors = append(errors, fmt.Errorf("missing remote service for candidate host: %q", node.ID))
		}
	}
	p.mu.Unlock()

	// FIXME: Node should already be registered at this point?
	node := store.Node{
		ID:       store.NodeID(nodeID),
		Kind:     kind,
		LastSeen: time.Now(),
		IsHost:   false,
	}
	// TODO: Connect with balance out of band
	if err := p.Store.SetNode(node, store.Account("")); err != nil {
		return nil, err
	}

	accepted := make([]store.Node, 0, len(remotes))
	callCtx, cancel := context.WithTimeout(ctx, poolWhitelistTimeout)

	// Parallelize whitelist, return any hosts that respond within the timeout.
	errChan := make(chan error)
	acceptChan := make(chan store.Node)

	for _, remote := range remotes {
		go func(service jsonrpc2.Service, node store.Node) {
			if err := service.Call(callCtx, nil, "vipnode_whitelist", nodeID); err != nil {
				errChan <- err
			} else {
				acceptChan <- node
			}
		}(remote.Service, remote.Node)
	}

	for i := len(remotes); i > 0; i-- {
		select {
		case node := <-acceptChan:
			accepted = append(accepted, node)
		case err := <-errChan:
			errors = append(errors, err)
		}
	}
	cancel()

	if len(errors) > 0 {
		logger.Printf("New %q client: %s (%d hosts found, %d accepted) %s", kind, nodeID[:8], len(remotes), len(accepted), ErrConnectFailed{errors})
	} else {
		logger.Printf("New %q client: %s (%d hosts found, %d accepted)", kind, nodeID[:8], len(remotes), len(accepted))
	}

	if len(accepted) >= 1 {
		return accepted, nil
	}

	if len(errors) > 0 {
		return nil, ErrConnectFailed{errors}
	}

	return nil, ErrNoHostNodes{len(r)}
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
