package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/tendermint/tendermint/rpc/client/http"
)

func main() {
	if err := Run(); err != nil {
		fmt.Printf("ERROR: %s", err.Error())
	}
}

func Run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: %s <url> [queryRange]", os.Args[0])
	}

	url := os.Args[1]
	c, err := http.New(url, "/websocket")
	if err != nil {
		return err
	}
	resp, err := c.Status(context.Background())
	if err != nil {
		return err
	}
	lastHeight := resp.SyncInfo.LatestBlockHeight
	chainID := resp.NodeInfo.Network
	queryRange := 100
	if len(os.Args) == 3 {
		queryRange, err = strconv.Atoi(os.Args[2])
		if err != nil {
			return err
		}
	}
	totalBlockSize := 0
	for i := lastHeight - int64(queryRange); i < lastHeight; i++ {
		if i < 2 {
			continue
		}
		resp, err := c.Block(context.Background(), &i)
		if err != nil {
			return err
		}

		protoBlock, err := resp.Block.ToProto()
		if err != nil {
			return err
		}

		totalBlockSize += protoBlock.Size()
	}
	gasLim := types.DefaultEstimateGas([]uint32{uint32(totalBlockSize)})
	fee := float64(gasLim) * appconsts.DefaultMinGasPrice
	fmt.Printf("%s from %d to %d (%d KB) would cost %f TIA\n",
		chainID,
		lastHeight-int64(queryRange),
		lastHeight,
		totalBlockSize/1024,
		fee/1e6,
	)
	return nil
}
