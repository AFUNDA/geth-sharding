// Package proposer defines all relevant functionality for a Proposer actor
// within the minimal sharding protocol.
package proposer

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/sharding"
	"github.com/ethereum/go-ethereum/sharding/mainchain"
	"github.com/ethereum/go-ethereum/sharding/p2p"
)

// Proposer holds functionality required to run a collation proposer
// in a sharded system. Must satisfy the Service interface defined in
// sharding/service.go.
type Proposer struct {
	client       *mainchain.SMCClient
	shardp2p     sharding.ShardP2P
	txpool       sharding.TXPool
	shardChainDb ethdb.Database
	shardID      int
}

// NewProposer creates a struct instance of a proposer service.
// It will have access to a mainchain client, a shardp2p network,
// and a shard transaction pool.
func NewProposer(client *mainchain.SMCClient, shardp2p sharding.ShardP2P, txpool sharding.TXPool, shardChainDb ethdb.Database, shardID int) (*Proposer, error) {
	// Initializes a  directory persistent db.
	return &Proposer{client, shardp2p, txpool, shardChainDb, shardID}, nil
}

// Start the main loop for proposing collations.
func (p *Proposer) Start() error {
	log.Info("Starting proposer service")
	go p.proposeCollations()
	go p.handleCollationBodyRequests()
	go simulateNotaryRequests()
	return nil
}

// Stop the main loop for proposing collations.
func (p *Proposer) Stop() error {
	log.Info("Stopping proposer service")
	return nil
}

// handleCollationBodyRequests subscribes to messages from the shardp2p
// network and responds to a specific peer that requested the body using
// the feed exposed by the p2p server's API.
func (p *Proposer) handleCollationBodyRequests() {
	feed := p.shardp2p.Feed(sharding.CollationBodyRequest{})
	ch := make(chan p2p.Message, 100)
	sub := feed.Subscribe(ch)
	// TODO: close chan and unsubscribe in Stop()
	for {
		req := <-ch
		// TODO: fetch data from db
		// res := buildResponse(req.Data)
		// Reply to that specific peer only.
		p2pserver.Send(nil, req.Peer)
	}
}

// proposeCollations is the main event loop of a proposer service that listens for
// incoming transactions and adds them to the SMC.
func (p *Proposer) proposeCollations() {

	// TODO: Receive TXs from shard TX generator or TXpool (Github Issues 153 and 161)
	var txs []*types.Transaction
	for i := 0; i < 10; i++ {
		data := make([]byte, 1024)
		rand.Read(data)
		txs = append(txs, types.NewTransaction(0, common.HexToAddress("0x0"),
			nil, 0, nil, data))
	}

	// Get current block number.
	blockNumber, err := p.client.ChainReader().BlockByNumber(context.Background(), nil)
	if err != nil {
		log.Error(fmt.Sprintf("Could not fetch current block number: %v", err))
		return
	}
	period := new(big.Int).Div(blockNumber.Number(), big.NewInt(sharding.PeriodLength))

	// Create collation.
	collation, err := createCollation(p.client, big.NewInt(int64(p.shardID)), period, txs)
	if err != nil {
		log.Error(fmt.Sprintf("Could not create collation: %v", err))
		return
	}

	// Check SMC if we can submit header before addHeader
	canAdd, err := checkHeaderAdded(p.client, big.NewInt(int64(p.shardID)), period)
	if err != nil {
		log.Error(fmt.Sprintf("Could not check if we can submit header: %v", err))
		return
	}
	if canAdd {
		addHeader(p.client, collation)
	}

	return nil
}

// Stop the main loop for proposing collations.
func (p *Proposer) Stop() error {
	log.Info(fmt.Sprintf("Stopping proposer service in shard %d", p.shardID))
	return nil
}
