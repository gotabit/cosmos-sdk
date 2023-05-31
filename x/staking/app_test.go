package staking_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankKeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingKeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	priv1 = secp256k1.GenPrivKey()
	addr1 = sdk.AccAddress(priv1.PubKey().Address())
	priv2 = secp256k1.GenPrivKey()
	addr2 = sdk.AccAddress(priv2.PubKey().Address())

	valKey          = ed25519.GenPrivKey()
	commissionRates = types.NewCommissionRates(math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec())
)

func TestStakingMsgs(t *testing.T) {
	genTokens := sdk.TokensFromConsensusPower(42, sdk.DefaultPowerReduction)
	bondTokens := sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)
	genCoin := sdk.NewCoin(sdk.DefaultBondDenom, genTokens)
	bondCoin := sdk.NewCoin(sdk.DefaultBondDenom, bondTokens)

	acc1 := &authtypes.BaseAccount{Address: addr1.String()}
	acc2 := &authtypes.BaseAccount{Address: addr2.String()}
	accs := []simtestutil.GenesisAccount{
		{GenesisAccount: acc1, Coins: sdk.Coins{genCoin}},
		{GenesisAccount: acc2, Coins: sdk.Coins{genCoin}},
	}

	var (
		bankKeeper    bankKeeper.Keeper
		stakingKeeper *stakingKeeper.Keeper
	)

	startupCfg := simtestutil.DefaultStartUpConfig()
	startupCfg.GenesisAccounts = accs

	app, err := simtestutil.SetupWithConfiguration(
		depinject.Configs(
			testutil.AppConfig,
			depinject.Supply(log.NewNopLogger()),
		),
		startupCfg, &bankKeeper, &stakingKeeper)
	require.NoError(t, err)
	ctxCheck := app.BaseApp.NewContext(true)

	require.True(t, sdk.Coins{genCoin}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr1)))
	require.True(t, sdk.Coins{genCoin}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr2)))

	// create validator
	description := types.NewDescription("foo_moniker", "", "", "", "")
	createValidatorMsg, err := types.NewMsgCreateValidator(
		sdk.ValAddress(addr1), valKey.PubKey(), bondCoin, description, commissionRates, math.OneInt(),
	)
	require.NoError(t, err)

	header := cmtproto.Header{Height: app.LastBlockHeight() + 1}
	txConfig := moduletestutil.MakeTestTxConfig()
	_, _, err = simtestutil.SignCheckDeliver(t, txConfig, app.BaseApp, header, []sdk.Msg{createValidatorMsg}, "", []uint64{0}, []uint64{0}, true, true, priv1)
	require.NoError(t, err)
	require.True(t, sdk.Coins{genCoin.Sub(bondCoin)}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr1)))

	app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: app.LastBlockHeight() + 1})
	ctxCheck = app.BaseApp.NewContext(true)
	validator, found := stakingKeeper.GetValidator(ctxCheck, sdk.ValAddress(addr1))
	require.True(t, found)
	require.Equal(t, sdk.ValAddress(addr1).String(), validator.OperatorAddress)
	require.Equal(t, types.Bonded, validator.Status)
	require.True(math.IntEq(t, bondTokens, validator.BondedTokens()))

	app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: app.LastBlockHeight() + 1})

	// edit the validator
	description = types.NewDescription("bar_moniker", "", "", "", "")
	editValidatorMsg := types.NewMsgEditValidator(sdk.ValAddress(addr1), description, nil, nil)

	header = cmtproto.Header{Height: app.LastBlockHeight() + 1}
	_, _, err = simtestutil.SignCheckDeliver(t, txConfig, app.BaseApp, header, []sdk.Msg{editValidatorMsg}, "", []uint64{0}, []uint64{1}, true, true, priv1)
	require.NoError(t, err)

	ctxCheck = app.BaseApp.NewContext(true)
	validator, found = stakingKeeper.GetValidator(ctxCheck, sdk.ValAddress(addr1))
	require.True(t, found)
	require.Equal(t, description, validator.Description)

	// delegate
	require.True(t, sdk.Coins{genCoin}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr2)))
	delegateMsg := types.NewMsgDelegate(addr2, sdk.ValAddress(addr1), bondCoin)

	header = cmtproto.Header{Height: app.LastBlockHeight() + 1}
	_, _, err = simtestutil.SignCheckDeliver(t, txConfig, app.BaseApp, header, []sdk.Msg{delegateMsg}, "", []uint64{1}, []uint64{0}, true, true, priv2)
	require.NoError(t, err)

	ctxCheck = app.BaseApp.NewContext(true)
	require.True(t, sdk.Coins{genCoin.Sub(bondCoin)}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr2)))
	_, found = stakingKeeper.GetDelegation(ctxCheck, addr2, sdk.ValAddress(addr1))
	require.True(t, found)

	// begin unbonding
	beginUnbondingMsg := types.NewMsgUndelegate(addr2, sdk.ValAddress(addr1), bondCoin)
	header = cmtproto.Header{Height: app.LastBlockHeight() + 1}
	_, _, err = simtestutil.SignCheckDeliver(t, txConfig, app.BaseApp, header, []sdk.Msg{beginUnbondingMsg}, "", []uint64{1}, []uint64{1}, true, true, priv2)
	require.NoError(t, err)

	// delegation should exist anymore
	ctxCheck = app.BaseApp.NewContext(true)
	_, found = stakingKeeper.GetDelegation(ctxCheck, addr2, sdk.ValAddress(addr1))
	require.False(t, found)

	// balance should be the same because bonding not yet complete
	require.True(t, sdk.Coins{genCoin.Sub(bondCoin)}.Equal(bankKeeper.GetAllBalances(ctxCheck, addr2)))
}
