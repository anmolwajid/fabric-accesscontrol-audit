package chaincode

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/cid"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

// Asset describes basic details of what makes up a simple asset
type Asset struct {
	ID             string `json:"ID"`
	Color          string `json:"Color"`
	Size           int    `json:"Size"`
	Owner          string `json:"Owner"`
	AppraisedValue int    `json:"AppraisedValue"`
	OwnerOrg       string `json:"OwnerOrg"` // org that created/owns the asset
}

// ===== helper: caller identity =====

func getClientMSPID(ctx contractapi.TransactionContextInterface) (string, error) {
	mspid, err := cid.GetMSPID(ctx.GetStub())
	if err != nil {
		return "", fmt.Errorf("failed to get client MSPID: %w", err)
	}
	return mspid, nil
}

func isClientAdmin(ctx contractapi.TransactionContextInterface) (bool, error) {
	// With Fabric CA/MSP, admin certs have attribute hf.Type=admin
	val, ok, err := cid.GetAttributeValue(ctx.GetStub(), "hf.Type")
	if err != nil {
		return false, err
	}
	return ok && val == "admin", nil
}

// ===== CRUD & queries =====

// CreateAsset issues a new asset to the world state.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("asset %s already exists", id)
	}
	mspid, err := getClientMSPID(ctx)
	if err != nil {
		return err
	}
	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
		OwnerOrg:       mspid,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(id, assetJSON)
}

// ReadAsset returns the asset stored in the world state with given id.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read asset %s: %w", id, err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("asset %s does not exist", id)
	}
	var asset Asset
	if err := json.Unmarshal(assetJSON, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// UpdateAsset updates an existing asset in the world state with matching id.
func (s *SmartContract) UpdateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	if err := s.assertCanModify(ctx, id); err != nil {
		return err
	}
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("asset %s does not exist", id)
	}
	// preserve OwnerOrg (ownership org)
	cur, _ := s.ReadAsset(ctx, id)
	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
		OwnerOrg:       cur.OwnerOrg,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(id, assetJSON)
}

// DeleteAsset deletes an given asset from the world state.
func (s *SmartContract) DeleteAsset(ctx contractapi.TransactionContextInterface, id string) error {
	if err := s.assertCanModify(ctx, id); err != nil {
		return err
	}
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("asset %s does not exist", id)
	}
	return ctx.GetStub().DelState(id)
}

// TransferAsset updates the owner field of asset with given id.
func (s *SmartContract) TransferAsset(ctx contractapi.TransactionContextInterface, id string, newOwner string) error {
	if err := s.assertCanModify(ctx, id); err != nil {
		return err
	}
	asset, err := s.ReadAsset(ctx, id)
	if err != nil {
		return err
	}
	asset.Owner = newOwner
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(id, assetJSON)
}

// GetAllAssets returns all assets found in world state.
func (s *SmartContract) GetAllAssets(ctx contractapi.TransactionContextInterface) ([]*Asset, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var assets []*Asset
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var asset Asset
		if err := json.Unmarshal(queryResponse.Value, &asset); err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}
	return assets, nil
}

// GetAssetHistory returns the history for a given asset id.
func (s *SmartContract) GetAssetHistory(ctx contractapi.TransactionContextInterface, id string) ([]map[string]interface{}, error) {
	iter, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var history []map[string]interface{}
	for iter.HasNext() {
		mod, err := iter.Next()
		if err != nil {
			return nil, err
		}
		var val Asset
		if mod.Value != nil {
			_ = json.Unmarshal(mod.Value, &val)
		}
		entry := map[string]interface{}{
			"txId":      mod.TxId,
			"isDelete":  mod.IsDelete,
			"timestamp": mod.Timestamp,
			"value":     val,
		}
		history = append(history, entry)
	}
	return history, nil
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read asset %s: %w", id, err)
	}
	return assetJSON != nil, nil
}

// ===== authorization guard =====

// allow if (caller is admin) OR (caller MSPID == asset.OwnerOrg)
func (s *SmartContract) assertCanModify(ctx contractapi.TransactionContextInterface, id string) error {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return fmt.Errorf("failed to read asset %s: %w", id, err)
	}
	if assetJSON == nil {
		return fmt.Errorf("asset %s does not exist", id)
	}
	var a Asset
	if err := json.Unmarshal(assetJSON, &a); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}
	mspid, err := getClientMSPID(ctx)
	if err != nil {
		return err
	}
	admin, err := isClientAdmin(ctx)
	if err != nil {
		return err
	}
	if admin || mspid == a.OwnerOrg {
		return nil
	}
	return fmt.Errorf("access denied: client org %s not allowed to modify asset owned by %s", mspid, a.OwnerOrg)
}
