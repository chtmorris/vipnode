package pool

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// Remote returns a RemotePool abstraction which proxies an RPC pool client but
// takes care of all the request signing.
func Remote(client jsonrpc2.Service, privkey *ecdsa.PrivateKey) *RemotePool {
	return &RemotePool{
		client:  client,
		privkey: privkey,
		nodeID:  discv5.PubkeyID(&privkey.PublicKey).String(),
	}
}

// Type assert for Pool implementation.
var _ Pool = &RemotePool{}

// RemotePool wraps a Pool with an RPC service and handles all the signging.
type RemotePool struct {
	client  jsonrpc2.Service
	privkey *ecdsa.PrivateKey
	nodeID  string
}

func (p *RemotePool) getNonce() int64 {
	return time.Now().UnixNano()
}

func (p *RemotePool) Host(ctx context.Context, kind string, payout string, nodeURI string) error {
	req := request.Request{
		Method:    "vipnode_host",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{kind, payout, nodeURI},
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return err
	}
	var result interface{}
	return p.client.Call(ctx, &result, req.Method, args...)
}

func (p *RemotePool) Connect(ctx context.Context, kind string) ([]store.Node, error) {
	req := request.Request{
		Method:    "vipnode_connect",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{kind},
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}
	var result []store.Node
	if err := p.client.Call(ctx, &result, req.Method, args...); err != nil {
		return nil, err
	}

	return result, nil
}

func (p *RemotePool) Disconnect(ctx context.Context) error {
	req := request.Request{
		Method: "vipnode_disconnect",
		NodeID: p.nodeID,
		Nonce:  p.getNonce(),
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return err
	}
	var result interface{}
	return p.client.Call(ctx, &result, req.Method, args...)
}

func (p *RemotePool) Update(ctx context.Context, peers []string) (*UpdateResponse, error) {
	req := request.Request{
		Method:    "vipnode_update",
		NodeID:    p.nodeID,
		Nonce:     p.getNonce(),
		ExtraArgs: []interface{}{peers},
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}

	var result UpdateResponse
	if err := p.client.Call(ctx, &result, req.Method, args...); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *RemotePool) Withdraw(ctx context.Context) error {
	req := request.Request{
		Method: "vipnode_withdraw",
		NodeID: p.nodeID,
		Nonce:  p.getNonce(),
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return err
	}
	var result interface{}
	return p.client.Call(ctx, &result, req.Method, args...)
}
