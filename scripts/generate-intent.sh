#!/bin/bash

set -e

WORK_DIR="${1:-/tmp/op-deployer-test}"

echo "🔧 Generating intent.toml with test addresses..."

# Anvil test addresses (deterministic)
DEPLOYER_ADDR="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"    # Account 0
BATCHER_ADDR="0x70997970C51812dc3A010C7d01b50e0d17dc79C8"     # Account 1  
PROPOSER_ADDR="0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"    # Account 2
ADMIN_ADDR="0x90F79bf6EB2c4f870365E785982E1f101E93b906"      # Account 3

cat > "${WORK_DIR}/intent.toml" << EOF
configType = "standard"
l1ChainID = 11155111
opcmAddress = "0x44c191ce5ce35131e703532af75fa9ca221e2398"
fundDevAccounts = false
l1ContractsLocator = "tag://op-contracts/v4.0.0-rc.7"
l2ContractsLocator = "tag://op-contracts/v4.0.0-rc.7"

[[chains]]
  id = "0x0000000000000000000000000000000000000000000000000000000000000309"
  baseFeeVaultRecipient = "${ADMIN_ADDR}"
  l1FeeVaultRecipient = "${ADMIN_ADDR}"
  sequencerFeeVaultRecipient = "${ADMIN_ADDR}"
  eip1559DenominatorCanyon = 250
  eip1559Denominator = 50
  eip1559Elasticity = 6
  operatorFeeScalar = 0
  operatorFeeConstant = 0
  [chains.roles]
    l1ProxyAdminOwner = "${ADMIN_ADDR}"
    l2ProxyAdminOwner = "${ADMIN_ADDR}"
    systemConfigOwner = "${ADMIN_ADDR}"
    unsafeBlockSigner = "${BATCHER_ADDR}"
    batcher = "${BATCHER_ADDR}"
    proposer = "${PROPOSER_ADDR}"
    challenger = "${ADMIN_ADDR}"
EOF

echo "✅ Intent file generated successfully!"
echo "📋 Addresses used:"
echo "  Admin/Owner: ${ADMIN_ADDR}"
echo "  Batcher:     ${BATCHER_ADDR}"
echo "  Proposer:    ${PROPOSER_ADDR}" 