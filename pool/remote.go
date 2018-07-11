package pool

import (
	"context"
	"crypto/ecdsa"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/vipnode/ethboot/forked/discv5"
	"github.com/vipnode/vipnode/request"
)

// Remote returns a RemotePool abstraction which proxies an RPC pool client but
// takes care of all the request signing.
func Remote(client *rpc.Client, privkey *ecdsa.PrivateKey) *RemotePool {
	return &RemotePool{
		client:  client,
		privkey: privkey,
		nodeID:  discv5.PubkeyID(&privkey.PublicKey).String(),
	}
}

// RemotePool wraps a Pool with an RPC service and handles all the signging.
type RemotePool struct {
	client  *rpc.Client
	privkey *ecdsa.PrivateKey
	nodeID  string
	nonce   int64
}

func (p *RemotePool) getNonce() int64 {
	return atomic.AddInt64(&p.nonce, 1)
}

func (p *RemotePool) Connect(ctx context.Context, kind string) ([]HostNode, error) {
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
	var result []HostNode
	if err := p.client.CallContext(ctx, &result, req.Method, args...); err != nil {
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
	return p.client.CallContext(ctx, &result, req.Method, args...)
}

func (p *RemotePool) Update(ctx context.Context, sig string, nodeID string, nonce int, peers []string) (*Balance, error) {
	req := request.Request{
		Method: "vipnode_update",
		NodeID: p.nodeID,
		Nonce:  p.getNonce(),
	}

	args, err := req.SignedArgs(p.privkey)
	if err != nil {
		return nil, err
	}

	var result Balance
	if err := p.client.CallContext(ctx, &result, req.Method, args...); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *RemotePool) Withdraw(ctx context.Context, sig string, nodeID string, nonce int) error {
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
	return p.client.CallContext(ctx, &result, req.Method, args...)
}
