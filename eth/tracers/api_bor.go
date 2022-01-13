package tracers

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type BlockTraceResult struct {
	Transactions []*TxTraceResult
	Block        interface{}
}

type TxTraceResult struct {
	// Trace results produced by the tracer
	Result interface{} `json:"result,omitempty"`

	// Trace failure produced by the tracer
	Error string `json:"error,omitempty"`

	// IntermediateHash of the execution if succeeds
	IntermediateHash common.Hash
}

func (api *API) traceBlock2(ctx context.Context, block *types.Block, config *TraceConfig) (*BlockTraceResult, error) {
	if block.NumberU64() == 0 {
		return nil, fmt.Errorf("genesis is not traceable")
	}

	res := &BlockTraceResult{
		Block: block,
	}

	// block object cannot be converted to JSON since much of the fields are non-public
	blockFields, err := ethapi.RPCMarshalBlock(block, true, true, api.backend.ChainConfig())
	if err != nil {
		return nil, err
	}
	res.Block = blockFields

	parent, err := api.blockByNumberAndHash(ctx, rpc.BlockNumber(block.NumberU64()-1), block.ParentHash())
	if err != nil {
		return nil, err
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	statedb, err := api.backend.StateAtBlock(ctx, parent, reexec, nil, true)
	if err != nil {
		return nil, err
	}

	// Execute all the transaction contained within the block concurrently
	var (
		signer             = types.MakeSigner(api.backend.ChainConfig(), block.Number())
		txs                = block.Transactions()
		deleteEmptyObjects = api.backend.ChainConfig().IsEIP158(block.Number())
	)

	blockCtx := core.NewEVMBlockContext(block.Header(), api.chainContext(ctx), nil)

	traceTxn := func(indx int, tx *types.Transaction) *TxTraceResult {
		message, _ := tx.AsMessage(signer, block.BaseFee())
		txContext := core.NewEVMTxContext(message)

		tracer := vm.NewStructLogger(config.LogConfig)

		// Run the transaction with tracing enabled.
		vmenv := vm.NewEVM(blockCtx, txContext, statedb, api.backend.ChainConfig(), vm.Config{Debug: true, Tracer: tracer, NoBaseFee: true})

		// Call Prepare to clear out the statedb access list
		// Not sure if we need to do this
		statedb.Prepare(tx.Hash(), indx)

		execRes, err := core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.Gas()))
		if err != nil {
			return &TxTraceResult{
				Error: err.Error(),
			}
		}

		returnVal := fmt.Sprintf("%x", execRes.Return())
		if len(execRes.Revert()) > 0 {
			returnVal = fmt.Sprintf("%x", execRes.Revert())
		}
		result := &ethapi.ExecutionResult{
			Gas:         execRes.UsedGas,
			Failed:      execRes.Failed(),
			ReturnValue: returnVal,
			StructLogs:  ethapi.FormatLogs(tracer.StructLogs()),
		}
		res := &TxTraceResult{
			Result:           result,
			IntermediateHash: statedb.IntermediateRoot(deleteEmptyObjects),
		}
		return res
	}

	for indx, tx := range txs {
		res.Transactions = append(res.Transactions, traceTxn(indx, tx))
	}
	return res, nil
}

type TraceBlockRequest struct {
	Number     int64
	Hash       string
	IsBadBlock bool
	Config     *TraceConfig
}

// If you use context as first parameter this function gets exposed automaticall on rpc endpoint
func (api *API) TraceBlock2(req *TraceBlockRequest) (*BlockTraceResult, error) {
	ctx := context.Background()

	block, err := api.blockByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	return api.traceBlock2(ctx, block, req.Config)
}