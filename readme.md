# Cheaper On Celestia

This is a simple fun tool which calculates how much posting the equivalent data (i.e. blocks) from any Cosmos chain on Celestia would cost.

## Usage

The tool is built in Go so to compile the binary simply run

```bash
go install
```

Then to run the tool, find the RPC and REST endpoint of any chain (a good source for this information is the [Cosmos Chain Registry](https://github.com/cosmos/chain-registry)) and run the following command:

```bash
cheaper-on-celestia <RPC endpoint> <REST endpoint>
```

This will query the most recent 100 blocks. If you want to specify the query range, add the number of blocks to the end of the command.

The response will look something like:

```bash
cosmoshub-4 over the last 100 blocks (3197 KB & 10m18s) would cost 3557435 utia
The same blocks cost 1032234798 uatom
```