package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/gogo/protobuf/proto"
	rpc "github.com/tendermint/tendermint/rpc/client/http"
)

func main() {
	if err := Run(); err != nil {
		fmt.Printf("ERROR: %s", err.Error())
	}
}

func Run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: %s <RPC> <REST> [queryRange]\n", os.Args[0])
	}

	rpcEndpoint := os.Args[1]
	c, err := rpc.New(rpcEndpoint, "/websocket")
	if err != nil {
		return fmt.Errorf("failed to connect to RPC endpoint: %s", err.Error())
	}
	resp, err := c.Status(context.Background())
	if err != nil {
		return err
	}
	lastHeight := resp.SyncInfo.LatestBlockHeight
	chainID := resp.NodeInfo.Network
	restEndpoint := os.Args[2]

	var annualProvisions float64
	switch chainID {
	case "osmosis-1":
		annualProvisions, err = getAnnualProvisionsOsmosis(restEndpoint)
		if err != nil {
			return err
		}
	default:
		annualProvisions, err = getAnnualProvisions(restEndpoint)
		if err != nil {
			return err
		}
	}

	queryRange := 100
	if len(os.Args) == 4 {
		queryRange, err = strconv.Atoi(os.Args[3])
		if err != nil {
			return err
		}
	}
	var (
		fistBlockTime, lastBlockTime time.Time
		totalBlockSize               int
		gasLim                       uint64
		txFees                       int64
		denom                        string
	)

	startHeight := lastHeight - int64(queryRange) + 1
	if startHeight < 1 {
		startHeight = 1
	}

	for i := startHeight; i <= lastHeight; i++ {
		resp, err := c.Block(context.Background(), &i)
		if err != nil {
			return fmt.Errorf("failed to get block %d: %s", i, err.Error())
		}

		// skip over the empty blocks
		if len(resp.Block.Txs) == 0 {
			continue
		}

		if i == startHeight {
			fistBlockTime = resp.Block.Time
		}

		if i == lastHeight {
			lastBlockTime = resp.Block.Time
		}

		time.Sleep(100 * time.Millisecond)

		res, err := c.BlockResults(context.Background(), &i)
		if err != nil {
			return fmt.Errorf("failed to get block results for block %d: %s", i, err.Error())
		}

		if len(res.TxsResults) != len(resp.Block.Txs) {
			return fmt.Errorf("mismatch between block txs and tx results (%d != %d)", len(res.TxsResults), len(resp.Block.Txs))
		}

		for idx, txRaw := range resp.Block.Txs {
			var t tx.Tx

			// ignore the fee for transactions that did not succeed
			if res.TxsResults[idx].Code != 0 {
				continue
			}

			err := proto.Unmarshal(txRaw, &t)
			if err != nil {
				return fmt.Errorf("failed to unmarshal tx %d: %s", idx, err.Error())
			}

			if denom == "" {
				denom = t.AuthInfo.Fee.Amount[0].Denom
			}

			txFees += t.AuthInfo.Fee.Amount.AmountOf(denom).Int64()
		}

		protoBlock, err := resp.Block.ToProto()
		if err != nil {
			return err
		}

		// we don't include commits in the size as they would not be part of
		// a rollup
		dataSize := (protoBlock.Size() - protoBlock.LastCommit.Size())

		totalBlockSize += dataSize

		// we assume each block corresponds to one blob in a PFB
		gasLim += types.DefaultEstimateGas([]uint32{uint32(dataSize)})
	}
	duration := lastBlockTime.Sub(fistBlockTime)

	inflation := annualProvisions * float64(duration.Milliseconds()) / (1000 * 60 * 60 * 24 * 365)
	fmt.Println("duration", duration.String(), "annualProvisions", int64(annualProvisions), "inflation", int64(inflation), "txFees", txFees)

	totalCost := txFees + int64(inflation)

	fee := float64(gasLim) * appconsts.DefaultMinGasPrice
	fmt.Printf("%s over the last %d blocks (%d KB & %s) would cost %d utia\nThe same blocks cost %d %s\n",
		chainID,
		lastHeight-startHeight+1,
		totalBlockSize/1024,
		duration.Round(time.Second).String(),
		int64(fee),
		totalCost,
		denom,
	)
	return nil
}

func getAnnualProvisions(restEndpoint string) (float64, error) {
	query := fmt.Sprintf("%s/cosmos/mint/v1beta1/annual_provisions", restEndpoint)
	res, err := http.Get(query)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code from query %s: %d", query, res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var parsedResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &parsedResp); err != nil {
		return 0, fmt.Errorf("parsing response to %s: %w", query, err)
	}
	annualProvisions := parsedResp["annual_provisions"].(string)

	return strconv.ParseFloat(annualProvisions, 64)
}

// we assume that an epoch is a day
func getAnnualProvisionsOsmosis(restEndpoint string) (float64, error) {
	query := fmt.Sprintf("%s/osmosis/mint/v1beta1/epoch_provisions", restEndpoint)
	res, err := http.Get(query)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code from query %s: %d", query, res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var parsedResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &parsedResp); err != nil {
		return 0, fmt.Errorf("parsing response to %s: %w", query, err)
	}
	epochProvisions := parsedResp["epoch_provisions"].(string)

	provisions, err := strconv.ParseFloat(epochProvisions, 64)
	if err != nil {
		return 0, err
	}

	return provisions * 365, nil
}
