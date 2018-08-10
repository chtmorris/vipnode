package main

import (
	"reflect"
	"testing"

	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

func TestPoolClient(t *testing.T) {
	privkey := keygen.HardcodedKey(t)

	p := pool.New()
	poolserver, poolrpc := jsonrpc2.ServePipe()
	poolserver.Register("vipnode_", p)

	node := fakenode.Node("foo")
	remote := pool.Remote(poolrpc, privkey)
	client := client.Client{
		EthNode: node,
		Pool:    remote,
	}

	if err := client.Connect(); err.Error() != pool.ErrNoHostNodes.Error() {
		t.Errorf("expected ErrNoHostNodes, got: %s", err)
	}

	if err := p.Store.SetHostNode(store.HostNode{URI: "foo"}); err != nil {
		t.Fatal(err)
	}

	if err := client.Connect(); err != nil {
		t.Error(err)
	}

	expected := fakenode.Calls{
		fakenode.Call("ConnectPeer", "foo"),
	}
	if !reflect.DeepEqual(node.Calls, expected) {
		t.Errorf("got %q; want %q", node.Calls, expected)
	}
}
