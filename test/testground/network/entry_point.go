package network

import (
	"fmt"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/test/testground/compositions"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// EntryPoint is the universal entry point for all role based tests.
func EntryPoint(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	kr := keyring.NewInMemory(ecfg.Codec)
	_, mn, err := kr.NewMnemonic("testground", keyring.English, "", "", hd.Secp256k1)
	if err != nil {
		return err
	}
	if mn == "" {
		runenv.RecordFailure(err)
		return fmt.Errorf("mnemonic is empty")
	}

	runenv.RecordMessage("starting mnemonic: %d", len(mn))

	initCtx, ctx, cancel, err := compositions.InitTest(runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}
	defer cancel()

	runenv.RecordMessage(fmt.Sprintf("testground entry point: seq %d", initCtx.GlobalSeq))

	// publish and download the ip addresses of all nodes
	statuses, err := SyncStatus(ctx, runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}

	runenv.RecordMessage("statuses: %v", statuses)

	// determine roles based only on the global sequence number. This allows for
	// us to deterministically calculate the IP addresses of each node.
	role, err := NewRole(runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}

	// The plan step is responsible for creating and distributing all network
	// configurations including the genesis, keys, node types, topology, etc
	// using the parameters defined in the manifest and plan toml files. The
	// single "leader" role performs creation and publishing of the configs,
	// while the "follower" roles download the configs from the leader.
	err = role.Plan(ctx, statuses, runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}

	// The execute step is responsible for starting the node and/or running any
	// tests.
	err = role.Execute(ctx, runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}

	// The retro step is responsible for collecting any data from the node and/or
	// running any retrospective tests or benchmarks.
	err = role.Retro(ctx, runenv, initCtx)
	if err != nil {
		runenv.RecordFailure(err)
		initCtx.SyncClient.MustSignalAndWait(ctx, FailedState, runenv.TestInstanceCount)
		return err
	}

	// signal that the test has completed successfully
	runenv.RecordSuccess()

	return err
}
