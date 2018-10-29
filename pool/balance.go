package pool

import (
	"fmt"
	"math/big"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

type BalanceManager interface {
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
	// TODO: OnConnect, OnDisconnect, etc? OnConnect would be useful for time-based trials.
	// TODO: Support error type that forces a disconnect (eg. trial expired?)
}

type payPerInterval struct {
	Store             store.Store
	Interval          time.Duration
	CreditPerInterval big.Int
}

// OnUpdate takes a node instance (with a Lastseen timestamp of the previous
// update) and the current active peers.
func (b *payPerInterval) OnUpdate(node store.Node, peers []store.Node) (store.Balance, error) {
	if node.IsHost {
		// We ignore host updates, only update balance on client updates. If
		// client fails to update, then the host will disconnect.
		return b.Store.GetNodeBalance(node.ID)
	}
	if b.Interval <= 0 || b.CreditPerInterval.Cmp(new(big.Int)) == 0 {
		// FIXME: Ideally this should be caught earlier. Maybe move to an earlier On* callback once we have more. Also check to make sure the values are big enough for the int64/float64 math.
		return store.Balance{}, fmt.Errorf("payPerInterval: Invalid interval settings: %d per %s", &b.CreditPerInterval, b.Interval)
	}
	delta := big.NewInt(int64(time.Now().Sub(node.LastSeen).Seconds()))
	interval := big.NewInt(int64(b.Interval.Seconds()))
	total := new(big.Int)
	for _, peer := range peers {
		credit := new(big.Int).Mul(delta, &b.CreditPerInterval).Div(delta, interval)
		b.Store.AddNodeBalance(peer.ID, credit)
		total.Add(total, credit)
	}
	if err := b.Store.AddNodeBalance(node.ID, new(big.Int).Neg(total)); err != nil {
		return store.Balance{}, err
	}
	return b.Store.GetNodeBalance(node.ID)
}
