package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client"
	"github.com/centrifuge/go-substrate-rpc-client/rpc/state"
	"github.com/centrifuge/go-substrate-rpc-client/signature"
	"github.com/centrifuge/go-substrate-rpc-client/types"
	"github.com/decred/base58"
)

func InitAutoPayout(ctx context.Context, stash, hotWallet, unit string, decimals int, listeners []Listener) error {
	api, err := gsrpc.NewSubstrateAPI("ws://127.0.0.1:9944")
	if err != nil {
		return err
	}

	kr, err := signature.KeyringPairFromSecret(hotWallet, "")
	if err != nil {
		return err
	}

	accountID := getAccountID(stash)
	go listenForEraPayout(ctx, api, func(block types.Hash, eraIndex types.U32) {
		unclaimed, err := fetchUnclaimedEra(api, accountID)
		if err != nil {
			sendMessage(fmt.Sprintf("Failed to fetch unclaimed eras: %v", err), listeners)
			return
		}

		batches := batchUnclaimed(10, unclaimed)
		for _, batch := range batches {
			err := payout(api, accountID, batch, kr)
			if err != nil {
				sendMessage(fmt.Sprintf("Failed to init payout for Eras(%d): %v", batch, err), listeners)
			}
		}
	})

	go listenForPayoutReward(ctx, api, accountID, func(block types.Hash, stash types.AccountID,
		amount types.U128) {
		payout := amount.Div(amount.Int, big.NewInt(1).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
		msg := fmt.Sprintf("Reward received: %s %s", payout.String(), unit)
		sendMessage(msg, listeners)
	})

	return nil
}

func sendMessage(msg string, listeners []Listener) {
	for _, l := range listeners {
		l.SendMessage(msg)
	}
}

func payout(api *gsrpc.SubstrateAPI, stash types.AccountID, eras []types.U32, kr signature.KeyringPair) error {
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return err
	}

	var calls []types.Call
	for _, era := range eras {
		c, err := types.NewCall(meta, "Staking.payout_stakers", stash, era)
		if err != nil {
			return err
		}

		calls = append(calls, c)
	}

	c, err := types.NewCall(meta, "Utility.batch", calls)
	if err != nil {
		return err
	}

	// Create the extrinsic
	ext := types.NewExtrinsic(c)
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return err
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return err
	}

	key, err := types.CreateStorageKey(meta, "System", "Account", kr.PublicKey, nil)
	if err != nil {
		return err
	}

	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return err
	}

	nonce := uint32(accountInfo.Nonce)
	o := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the transaction using Alice's default account
	err = ext.Sign(kr, o)
	if err != nil {
		return err
	}

	// Send the extrinsic
	sub, err := api.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		return err
	}

	defer sub.Unsubscribe()
	select {
	case c := <-sub.Chan():
		if c.IsInvalid {
			return errors.New("invalid extrinsic")
		}
		return nil
	case err := <-sub.Err():
		return err
	}
}

func getAccountID(address string) types.AccountID {
	data := base58.Decode(address)
	return types.NewAccountID(data[1 : len(data)-2])
}

func getEventSubscription(api *gsrpc.SubstrateAPI) (
	sub *state.StorageSubscription,
	meta *types.Metadata,
	key types.StorageKey, err error) {
	meta, err = api.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, nil, nil, err
	}

	// Subscribe to system events via storage
	key, err = types.CreateStorageKey(meta, "System", "Events", nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	sub, err = api.RPC.State.SubscribeStorageRaw([]types.StorageKey{key})
	return sub, meta, key, err
}

func listenForEraPayout(ctx context.Context, api *gsrpc.SubstrateAPI, onEraFinish func(block types.Hash,
	eraIndex types.U32)) error {
	sub, meta, key, err := getEventSubscription(api)
	if err != nil {
		return err
	}

	defer sub.Unsubscribe()

	// outer for loop for subscription notifications
	for {

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			return err
		case set := <-sub.Chan():
			// inner loop for the changes within one of those notifications
			for _, chng := range set.Changes {
				if !types.Eq(chng.StorageKey, key) || !chng.HasStorageData {
					// skip, we are only interested in events with content
					continue
				}

				// Decode the event records
				events := types.EventRecords{}
				err = types.EventRecordsRaw(chng.StorageData).DecodeEventRecords(meta, &events)
				if err != nil {
					return err
				}

				for _, e := range events.Staking_EraPayout {
					onEraFinish(set.Block, e.EraIndex)
				}
			}
		}
	}
}

func listenForPayoutReward(
	ctx context.Context,
	api *gsrpc.SubstrateAPI,
	stash types.AccountID,
	onReward func(block types.Hash, stash types.AccountID, amount types.U128)) error {
	sub, meta, key, err := getEventSubscription(api)
	if err != nil {
		return err
	}

	defer sub.Unsubscribe()

	// outer for loop for subscription notifications
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			return err
		case set := <-sub.Chan():
			// inner loop for the changes within one of those notifications
			for _, chng := range set.Changes {
				if !types.Eq(chng.StorageKey, key) || !chng.HasStorageData {
					// skip, we are only interested in events with content
					continue
				}

				// Decode the event records
				events := types.EventRecords{}
				err = types.EventRecordsRaw(chng.StorageData).DecodeEventRecords(meta, &events)
				if err != nil {
					return err
				}

				for _, e := range events.Staking_Reward {
					if !bytes.Equal(e.Stash[:], stash[:]) {
						continue
					}

					onReward(set.Block, e.Stash, e.Amount)
				}
			}
		}
	}
}

func fetchUnclaimedEra(api *gsrpc.SubstrateAPI, stash types.AccountID) ([]types.U32, error) {
	controller, err := bonded(api, stash)
	if err != nil {
		return nil, err
	}

	claimed, err := fetchClaimed(api, controller)
	if err != nil {
		return nil, err
	}

	activeEra, err := activeEra(api)
	if err != nil {
		return nil, err
	}

	depth := historyDepth(api, 84)
	claimedMap := make(map[types.U32]bool)
	for _, c := range claimed {
		claimedMap[c] = true
	}

	var unclaimed []types.U32
	for i := activeEra - depth - 1; i < activeEra; i++ {
		exposure, err := fetchExposure(api, i, stash)
		if err != nil {
			continue
		}

		own := big.Int(exposure.Own)
		zero := big.NewInt(0)
		if own.Cmp(zero) != 1 || claimedMap[i] {
			continue
		}

		unclaimed = append(unclaimed, i)
	}

	return unclaimed, nil
}

func batchUnclaimed(maxErasPerBatch int, eras []types.U32) [][]types.U32 {
	if len(eras) <= maxErasPerBatch {
		return [][]types.U32{eras}
	}

	var res [][]types.U32
	var cur []types.U32
	for _, era := range eras {
		cur = append(cur, era)
		if len(cur) == maxErasPerBatch {
			res = append(res, append([]types.U32{}, cur...))
			cur = nil
		}
	}

	if len(cur) > 0 {
		res = append(res, cur)
	}

	return res
}

type StakingLedger struct {
	Stash         types.AccountID
	Total, Active types.UCompact
	Unlocking     []struct {
		Value types.UCompact
		Era   types.U32
	}
	ClaimedRewards []types.U32
}

type Exposure struct {
	Total, Own types.UCompact
	Others     []struct {
		Who   types.AccountID
		Value types.UCompact
	}
}

func historyDepth(api *gsrpc.SubstrateAPI, or types.U32) types.U32 {
	var depth types.U32
	err := fetchStorage(api, "Staking", "HistoryDepth", nil, nil, &depth)
	if err != nil {
		return or
	}

	return depth
}

func bonded(api *gsrpc.SubstrateAPI, stash types.AccountID) (acc types.AccountID, err error) {
	var controller types.AccountID
	return controller, fetchStorage(api, "Staking", "Bonded", stash[:], nil, &controller)
}

func fetchClaimed(api *gsrpc.SubstrateAPI, controller types.AccountID) (unclaimed []types.U32, err error) {
	var res StakingLedger
	return res.ClaimedRewards, fetchStorage(api, "Staking", "Ledger", controller[:], nil, &res)
}

func activeEra(api *gsrpc.SubstrateAPI) (types.U32, error) {
	var eraInfo struct {
		Era   types.U32
		Start types.OptionU64
	}
	return eraInfo.Era, fetchStorage(api, "Staking", "ActiveEra", nil, nil, &eraInfo)
}

func fetchExposure(api *gsrpc.SubstrateAPI, era types.U32, stash types.AccountID) (Exposure, error) {
	var res Exposure
	eraBytes, err := types.EncodeToBytes(era)
	if err != nil {
		return res, err
	}

	return res, fetchStorage(api, "Staking", "ErasStakers", eraBytes, stash[:], &res)
}

func fetchStorage(api *gsrpc.SubstrateAPI, prefix, method string, arg1, arg2 []byte, target interface{}) error {
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return err
	}

	key, err := types.CreateStorageKey(meta, prefix, method, arg1, arg2)
	if err != nil {
		return err
	}

	ok, err := api.RPC.State.GetStorageLatest(key, target)
	if err != nil || !ok {
		return fmt.Errorf("failed to fetch storage: %w", err)
	}

	return nil
}
