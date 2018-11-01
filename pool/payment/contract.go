package payment

import (
	"context"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/vipnode/vipnode-contract/go/vipnodepool"
	"github.com/vipnode/vipnode/pool/store"
)

var zeroInt = &big.Int{}

// ErrDepositTimelocked is returned when a balance is checked but the deposit
// is timelocked.
var ErrDepositTimelocked = errors.New("deposit is timelocked")

// ContractPayment returns an abstraction around a vipnode pool payment
// contract. Contract implements store.NodeBalanceStore.
func ContractPayment(storeDriver store.AccountStore, address common.Address, backend bind.ContractBackend) (*contractPayment, error) {
	contract, err := vipnodepool.NewVipnodePool(address, backend)
	if err != nil {
		return nil, err
	}
	return &contractPayment{
		store:    storeDriver,
		contract: contract,
		backend:  backend,
	}, nil
}

var _ store.BalanceStore = &contractPayment{}

// ContractPayment uses the github.com/vipnode/vipnode-contract smart contract for payment.
type contractPayment struct {
	store    store.AccountStore
	contract *vipnodepool.VipnodePool
	backend  bind.ContractBackend
}

// GetNodeBalance proxies the normal store implementation
// by adding the contract deposit to the resulting balance.
func (p *contractPayment) GetNodeBalance(nodeID store.NodeID) (store.Balance, error) {
	balance, err := p.store.GetNodeBalance(nodeID)
	if err != nil {
		return balance, err
	}

	if len(balance.Account) == 0 {
		// No account associated, probably on trial
		return balance, nil
	}

	// FIXME: Cache this, since it's pretty slow. Use SubscribeBalance to update the cache.
	deposit, err := p.GetBalance(balance.Account)
	if err != nil {
		return balance, err
	}
	balance.Deposit = *deposit
	return balance, nil
}

// AddNodeBalance proxies to the underlying store.BalanceStore
func (p *contractPayment) AddNodeBalance(nodeID store.NodeID, credit *big.Int) error {
	return p.store.AddNodeBalance(nodeID, credit)
}

// GetAccountBalance returns an account's balance, which includes the contract deposit.
func (p *contractPayment) GetAccountBalance(account store.Account) (store.Balance, error) {
	balance, err := p.store.GetAccountBalance(account)
	if err != nil {
		return balance, err
	}

	// FIXME: Cache this, since it's pretty slow. Use SubscribeBalance to update the cache.
	deposit, err := p.GetBalance(balance.Account)
	if err != nil {
		return balance, err
	}
	balance.Deposit = *deposit
	return balance, nil
}

// AddAccountBalance proxies to the underlying store.BalanceStore
func (p *contractPayment) AddAccountBalance(account store.Account, credit *big.Int) error {
	return p.store.AddAccountBalance(account, credit)
}

func (p *contractPayment) SubscribeBalance(ctx context.Context, handler func(account store.Account, amount *big.Int)) error {
	sink := make(chan *vipnodepool.VipnodePoolBalance, 1)
	sub, err := p.contract.WatchBalance(&bind.WatchOpts{
		Context: ctx,
	}, sink)
	if err != nil {
		return err
	}
	for {
		select {
		case balanceEvent := <-sink:
			account := store.Account(balanceEvent.Client.Hex())
			go handler(account, balanceEvent.Balance)
		case err := <-sub.Err():
			return err
		case <-ctx.Done():
			sub.Unsubscribe()
			return ctx.Err()
		}
	}
}

func (p *contractPayment) GetBalance(account store.Account) (*big.Int, error) {
	timer := time.Now()
	r, err := p.contract.Clients(&bind.CallOpts{Pending: true}, common.HexToAddress(string(account)))
	if err != nil {
		return nil, err
	}
	if r.TimeLocked.Cmp(zeroInt) != 0 {
		return nil, ErrDepositTimelocked
	}
	log.Printf("Retrieved balance for %s in %d: %d", account, time.Now().Sub(timer), r.Balance)
	return r.Balance, nil
}

func (p *contractPayment) Withdraw(account store.Account, amount *big.Int) (tx string, err error) {
	return "", errors.New("ContractPayment has not implemented Withdraw")
}
