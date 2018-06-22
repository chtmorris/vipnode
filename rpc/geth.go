package rpc

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/rpc"
)

const errCodeMethodNotFound = -32601

type codedError interface {
	error
	ErrorCode() int
}

type gethNode struct {
	client *rpc.Client
}

func (n *gethNode) CheckCompatible(ctx context.Context) error {
	// TODO: Make sure we have the necessary APIs available, maybe version check?
	var result interface{}
	err := n.client.CallContext(ctx, &result, "admin_addTrustedPeer", "")
	if err == nil {
		return errors.New("failed to detect compatibility")
	}
	if err, ok := err.(codedError); ok && err.ErrorCode() == errCodeMethodNotFound {
		return err
	}
	return nil
}

func (n *gethNode) ConnectPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_addPeer", nodeID)
}

func (n *gethNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_removePeer", nodeID)
}

func (n *gethNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_addTrustedPeer", nodeID)
}

func (n *gethNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_removeTrustedPeer", nodeID)
}

func (n *gethNode) Peers(ctx context.Context) ([]PeerInfo, error) {
	var peers []PeerInfo
	err := n.client.CallContext(ctx, &peers, "admin_peers")
	if err != nil {
		return nil, err
	}
	return peers, nil
}
