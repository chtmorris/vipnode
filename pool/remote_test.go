package pool

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/pool/store"
)

func TestRemotePool(t *testing.T) {
	pool := New()
	server, err := pool.ServeRPC()
	if err != nil {
		t.Fatal(err)
	}

	privkey := keygen.HardcodedKey(t)
	client := rpc.DialInProc(server)
	remote := Remote(client, privkey)

	// Add self to pool first, then let's see if we're advised to connect to
	// self (this probably should error at some point but good test for now).
	if err := pool.Store.SetHostNode(store.HostNode{URI: "foo"}); err != nil {
		t.Fatal("failed to add host node:", err)
	}

	hosts, err := remote.Connect(context.Background(), "geth")
	if err != nil {
		t.Error(err)
	}
	if len(hosts) == 0 {
		t.Fatal("no hosts")
	}

	if hosts[0].URI != "foo" {
		t.Errorf("invalid hosts result: %v", hosts)
	}
}
