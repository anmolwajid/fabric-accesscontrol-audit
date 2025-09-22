# Fabric Access Control + Audit Extension

This repository contains a modified version of the Hyperledger Fabric 
`asset-transfer-basic` chaincode with the following extensions:

- **Access Control:** Introduces `OwnerOrg` field and `assertCanModify` logic 
  so only the creating organization (or an admin) can update or delete an asset.
- **Auditability:** Adds a `GetAssetHistory` function to retrieve the complete 
  transaction history (including timestamps and TxIDs).
- **Transparency:** Validated that peers across multiple organizations always 
  return the same ledger state.

## 🛠️ Pre-requisites

- **OS:** Windows 10 with WSL2 (Ubuntu 22.04)
- **Tools:**
  - Docker & Docker Compose
  - Go (>=1.22.3)
  - Node.js (>=v22)
  - Git
- **Fabric Binaries:** Place under `fabric-samples/bin` (peer, orderer, configtxgen, etc.)

## 🚀 Setup Instructions

```bash
# Bring network down if already running
./network.sh down

# Start the network and create channel
./network.sh up createChannel -ca

# Deploy modified chaincode
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-go -ccl go
