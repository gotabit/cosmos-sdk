package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/errors"
)

var _ types.QueryServer = Keeper{}

// CurrentPlan implements the Query/CurrentPlan gRPC method
func (k Keeper) CurrentPlan(c context.Context, req *types.QueryCurrentPlanRequest) (*types.QueryCurrentPlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	plan, found, err := k.GetUpgradePlan(ctx)
	if err != nil {
		return nil, err
	}

	if !found {
		return &types.QueryCurrentPlanResponse{}, nil
	}

	return &types.QueryCurrentPlanResponse{Plan: &plan}, nil
}

// AppliedPlan implements the Query/AppliedPlan gRPC method
func (k Keeper) AppliedPlan(c context.Context, req *types.QueryAppliedPlanRequest) (*types.QueryAppliedPlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	applied, err := k.GetDoneHeight(ctx, req.Name)

	return &types.QueryAppliedPlanResponse{Height: applied}, err
}

// UpgradedConsensusState implements the Query/UpgradedConsensusState gRPC method
func (k Keeper) UpgradedConsensusState(c context.Context, req *types.QueryUpgradedConsensusStateRequest) (*types.QueryUpgradedConsensusStateResponse, error) { //nolint:staticcheck // we're using a deprecated call for compatibility
	ctx := sdk.UnwrapSDKContext(c)

	consState, found, err := k.GetUpgradedConsensusState(ctx, req.LastHeight)
	if err != nil {
		return nil, err
	}

	if !found {
		return &types.QueryUpgradedConsensusStateResponse{}, nil //nolint:staticcheck // we're using a deprecated call for compatibility
	}

	return &types.QueryUpgradedConsensusStateResponse{ //nolint:staticcheck // we're using a deprecated call for compatibility
		UpgradedConsensusState: consState,
	}, nil
}

// ModuleVersions implements the Query/QueryModuleVersions gRPC method
func (k Keeper) ModuleVersions(c context.Context, req *types.QueryModuleVersionsRequest) (*types.QueryModuleVersionsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	// check if a specific module was requested
	if len(req.ModuleName) > 0 {
		version, ok, err := k.getModuleVersion(ctx, req.ModuleName)
		if err != nil {
			return nil, err
		}

		if ok {
			// return the requested module
			res := []*types.ModuleVersion{{Name: req.ModuleName, Version: version}}
			return &types.QueryModuleVersionsResponse{ModuleVersions: res}, nil
		}
		// module requested, but not found
		return nil, errorsmod.Wrapf(errors.ErrNotFound, "x/upgrade: QueryModuleVersions module %s not found", req.ModuleName)
	}

	// if no module requested return all module versions from state
	mv, err := k.GetModuleVersions(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryModuleVersionsResponse{
		ModuleVersions: mv,
	}, nil
}

// Authority implements the Query/Authority gRPC method, returning the account capable of performing upgrades
func (k Keeper) Authority(c context.Context, req *types.QueryAuthorityRequest) (*types.QueryAuthorityResponse, error) {
	return &types.QueryAuthorityResponse{Address: k.authority}, nil
}
