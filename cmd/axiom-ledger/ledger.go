package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/axiomesh/axiom-kit/fileutil"
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/app"
	"github.com/axiomesh/axiom-ledger/internal/consensus/common"
	"github.com/axiomesh/axiom-ledger/internal/executor"
	"github.com/axiomesh/axiom-ledger/internal/ledger"
	"github.com/axiomesh/axiom-ledger/internal/ledger/genesis"
	"github.com/axiomesh/axiom-ledger/pkg/loggers"
	"github.com/axiomesh/axiom-ledger/pkg/repo"
)

var ledgerCMD = &cli.Command{
	Name:  "ledger",
	Usage: "The ledger manage commands",
	Subcommands: []*cli.Command{
		{
			Name:   "block",
			Usage:  "Get block info by number or hash, if not specified, get the latest",
			Action: getBlock,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "full",
					Aliases:  []string{"f"},
					Usage:    "additionally display transactions",
					Required: false,
				},
				&cli.Uint64Flag{
					Name:     "number",
					Aliases:  []string{"n"},
					Usage:    "block number",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "hash",
					Usage:    "block hash",
					Required: false,
				},
			},
		},
		{
			Name:   "chain-meta",
			Usage:  "Get latest chain meta info",
			Action: getLatestChainMeta,
		},
		{
			Name:   "simple-rollback",
			Usage:  "Rollback to the local block history height (by replaying transactions)",
			Action: simpleRollback,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:     "target-block-number",
					Aliases:  []string{"b"},
					Usage:    "rollback target block number, must be less than the current latest block height and greater than 1",
					Required: true,
				},
				&cli.BoolFlag{
					Name:     "force",
					Aliases:  []string{"f"},
					Usage:    "disable interactive confirmation and remove existing rollback storage directory of the same height",
					Required: false,
				},
			},
		},
	},
}

func getBlock(ctx *cli.Context) error {
	r, err := prepareRepo(ctx)
	if err != nil {
		return err
	}

	chainLedger, err := ledger.NewChainLedger(r, "")
	if err != nil {
		return fmt.Errorf("init chain ledger failed: %w", err)
	}

	var block *types.Block

	if ctx.IsSet("number") {
		block, err = chainLedger.GetBlock(ctx.Uint64("number"))
		if err != nil {
			return err
		}
	} else {
		if ctx.IsSet("hash") {
			block, err = chainLedger.GetBlockByHash(types.NewHashByStr(ctx.String("hash")))
			if err != nil {
				return err
			}
		} else {
			block, err = chainLedger.GetBlock(chainLedger.GetChainMeta().Height)
			if err != nil {
				return err
			}
		}
	}

	bloom, _ := block.BlockHeader.Bloom.ETHBloom().MarshalText()
	blockInfo := map[string]any{
		"number":           block.BlockHeader.Number,
		"hash":             block.Hash().String(),
		"state_root":       block.BlockHeader.StateRoot.String(),
		"tx_root":          block.BlockHeader.TxRoot.String(),
		"receipt_root":     block.BlockHeader.ReceiptRoot.String(),
		"parent_hash":      block.BlockHeader.ParentHash.String(),
		"timestamp":        block.BlockHeader.Timestamp,
		"epoch":            block.BlockHeader.Epoch,
		"bloom":            string(bloom),
		"gas_price":        block.BlockHeader.GasPrice,
		"proposer_account": block.BlockHeader.ProposerAccount,
	}

	if ctx.Bool("full") {
		blockInfo["transactions"] = lo.Map(block.Transactions, func(item *types.Transaction, index int) string {
			return item.GetHash().String()
		})
	}

	return pretty(blockInfo)
}

func getLatestChainMeta(ctx *cli.Context) error {
	r, err := prepareRepo(ctx)
	if err != nil {
		return err
	}

	chainLedger, err := ledger.NewChainLedger(r, "")
	if err != nil {
		return fmt.Errorf("init chain ledger failed: %w", err)
	}

	meta := chainLedger.GetChainMeta()
	return pretty(meta)
}

func simpleRollback(ctx *cli.Context) error {
	r, err := prepareRepo(ctx)
	if err != nil {
		return err
	}

	chainLedger, err := ledger.NewChainLedger(r, "")
	if err != nil {
		return fmt.Errorf("init chain ledger failed: %w", err)
	}

	targetBlockNumber := ctx.Uint64("target-block-number")
	if targetBlockNumber <= 1 {
		return errors.New("target-block-number must be greater than 1")
	}
	chainMeta := chainLedger.GetChainMeta()
	if targetBlockNumber >= chainMeta.Height {
		fmt.Printf("target-block-number %d must be less than the current latest block height %d\n", targetBlockNumber, chainMeta.Height)
		return nil
	}
	rollbackDir := path.Join(r.RepoRoot, fmt.Sprintf("storage-rollback-%d", targetBlockNumber))

	force := ctx.Bool("force")
	if fileutil.Exist(rollbackDir) {
		if !force {
			return errors.Errorf("rollback dir %s already exists\n", rollbackDir)
		}
		if err := os.RemoveAll(rollbackDir); err != nil {
			return err
		}
	}

	targetBlockHash := chainLedger.GetBlockHash(targetBlockNumber).String()

	logger := loggers.Logger(loggers.App)
	if force {
		logger.Infof("current chain meta info, height: %d, hash: %s, will rollback to the target height %d, hash: %s, rollback storage dir: %s\n", chainMeta.Height, chainMeta.BlockHash, targetBlockNumber, targetBlockHash, rollbackDir)
	} else {
		logger.Infof("current chain meta info, height: %d, hash: %s, will rollback to the target height %d, hash: %s, rollback storage dir: %s, confirm? y/n\n", chainMeta.Height, chainMeta.BlockHash, targetBlockNumber, targetBlockHash, rollbackDir)
		var choice string
		if _, err := fmt.Scanln(&choice); err != nil {
			return err
		}
		if choice != "y" {
			return errors.New("interrupt by user")
		}
	}

	rollbackChainLedger, err := ledger.NewChainLedger(r, rollbackDir)
	if err != nil {
		return fmt.Errorf("init rollback chain ledger failed: %w", err)
	}
	rollbackStateLedger, err := ledger.NewStateLedger(r, rollbackDir)
	if err != nil {
		return fmt.Errorf("init rollback state ledger failed: %w", err)
	}
	rollbackLedger := &ledger.Ledger{
		ChainLedger: rollbackChainLedger,
		StateLedger: rollbackStateLedger,
	}
	if err := genesis.Initialize(&r.Config.Genesis, rollbackLedger); err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{
		"genesis block hash": rollbackLedger.ChainLedger.GetChainMeta().BlockHash,
	}).Info("Initialize genesis")

	e, err := executor.New(r, rollbackLedger)
	if err != nil {
		return fmt.Errorf("init executor failed: %w", err)
	}

	logger.Infof("rollback start, storage dir: %s", rollbackDir)
	blockCh := make(chan *common.CommitEvent, 100)
	go func() {
		for i := rollbackChainLedger.GetChainMeta().Height + 1; i <= targetBlockNumber; i++ {
			b, err := chainLedger.GetBlock(i)
			if err != nil {
				panic(errors.Wrapf(err, "failed to get block %d", i))
			}
			select {
			case <-ctx.Done():
				return
			case blockCh <- &common.CommitEvent{
				Block: b,
			}:
			}
		}
		blockCh <- nil
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case b := <-blockCh:
			if b == nil {
				return nil
			}
			var originBlockHash string
			if b.Block.Height()%10 == 0 || b.Block.Height() == targetBlockNumber {
				originBlockHash = b.Block.BlockHeader.Hash().String()
			}

			e.ExecuteBlock(b)

			// check hash
			if b.Block.Height()%10 == 0 || b.Block.Height() == targetBlockNumber {
				if originBlockHash != rollbackLedger.ChainLedger.GetChainMeta().BlockHash.String() {
					panic(fmt.Sprintf("inconsistent block %d hash, get %s, but want %s", b.Block.Height(), rollbackLedger.ChainLedger.GetChainMeta().BlockHash.String(), originBlockHash))
				}
			}
		}
	}
}

func prepareRepo(ctx *cli.Context) (*repo.Repo, error) {
	p, err := getRootPath(ctx)
	if err != nil {
		return nil, err
	}
	if !fileutil.Exist(filepath.Join(p, repo.CfgFileName)) {
		return nil, errors.New("axiom-ledger repo not exist")
	}

	r, err := repo.Load(p)
	if err != nil {
		return nil, err
	}

	fmt.Printf("%s-repo: %s\n", repo.AppName, r.RepoRoot)

	if err := app.PrepareAxiomLedger(r); err != nil {
		return nil, fmt.Errorf("prepare axiom-ledger failed: %w", err)
	}
	return r, nil
}

func pretty(d any) error {
	res, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(res))
	return nil
}
