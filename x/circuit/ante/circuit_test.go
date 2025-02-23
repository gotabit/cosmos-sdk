package ante_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	cbtypes "cosmossdk.io/x/circuit/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"

	"cosmossdk.io/x/circuit/ante"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	ctx           sdk.Context
	mockStoreKey  storetypes.StoreKey
	mockMsgURL    string
	mockclientCtx client.Context
	txBuilder     client.TxBuilder
}

type MockCircuitBreaker struct {
	isAllowed bool
}

func (m MockCircuitBreaker) IsAllowed(ctx sdk.Context, typeURL string) bool {
	return typeURL == "/cosmos.circuit.v1.MsgAuthorizeCircuitBreaker"
}

func initFixture(t *testing.T) *fixture {
	mockStoreKey := storetypes.NewKVStoreKey("test")
	encCfg := moduletestutil.MakeTestEncodingConfig(auth.AppModuleBasic{}, bank.AppModuleBasic{})
	mockclientCtx := client.Context{}.
		WithTxConfig(encCfg.TxConfig).
		WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{}))

	return &fixture{
		ctx:           testutil.DefaultContextWithDB(t, mockStoreKey, storetypes.NewTransientStoreKey("transient_test")).Ctx.WithBlockHeader(cmproto.Header{}),
		mockStoreKey:  mockStoreKey,
		mockMsgURL:    "test",
		mockclientCtx: mockclientCtx,
		txBuilder:     mockclientCtx.TxConfig.NewTxBuilder(),
	}
}

func TestCircuitBreakerDecorator(t *testing.T) {
	t.Parallel()
	f := initFixture(t)

	_, _, addr1 := testdata.KeyTestPubAddr()

	testcases := []struct {
		msg     sdk.Msg
		allowed bool
	}{
		{msg: &cbtypes.MsgAuthorizeCircuitBreaker{
			Grantee: "cosmos1fghij",
			Granter: "cosmos1abcde",
		}, allowed: true},
		{msg: testdata.NewTestMsg(addr1), allowed: false},
	}

	for _, tc := range testcases {
		// Circuit breaker is allowed to pass through all transactions
		circuitBreaker := &MockCircuitBreaker{true}
		// CircuitBreakerDecorator AnteHandler should always return success
		decorator := ante.NewCircuitBreakerDecorator(circuitBreaker)

		f.txBuilder.SetMsgs(tc.msg)
		tx := f.txBuilder.GetTx()

		_, err := decorator.AnteHandle(f.ctx, tx, false, func(ctx sdk.Context, tx sdk.Tx, simulate bool) (newCtx sdk.Context, err error) {
			return ctx, nil
		})

		if tc.allowed {
			require.NoError(t, err)
		} else {
			require.Equal(t, "tx type not allowed", err.Error())
		}
	}
}
