package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
)

// ContractDiscoveryService handles automatic discovery of OP Stack contract addresses
type ContractDiscoveryService struct {
	cache        map[string]*CachedNetworkAddresses
	cacheTimeout time.Duration
}

// CachedNetworkAddresses contains cached contract addresses with expiration
type CachedNetworkAddresses struct {
	Addresses *optimismv1alpha1.NetworkContractAddresses
	ExpiresAt time.Time
}

// NewContractDiscoveryService creates a new contract discovery service
func NewContractDiscoveryService(cacheTimeout time.Duration) *ContractDiscoveryService {
	return &ContractDiscoveryService{
		cache:        make(map[string]*CachedNetworkAddresses),
		cacheTimeout: cacheTimeout,
	}
}

// DiscoverContracts discovers contract addresses for the given OptimismNetwork
func (c *ContractDiscoveryService) DiscoverContracts(
	ctx context.Context,
	network *optimismv1alpha1.OptimismNetwork,
) (*optimismv1alpha1.NetworkContractAddresses, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s-%d", network.Spec.NetworkName, network.Spec.ChainID)
	if cached, exists := c.cache[cacheKey]; exists && !c.isCacheExpired(cached) {
		return cached.Addresses, nil
	}

	var addresses *optimismv1alpha1.NetworkContractAddresses
	var err error

	// Determine discovery method
	discoveryMethod := "auto"
	if network.Spec.ContractAddresses != nil && network.Spec.ContractAddresses.DiscoveryMethod != "" {
		discoveryMethod = network.Spec.ContractAddresses.DiscoveryMethod
	}

	switch discoveryMethod {
	case "auto":
		addresses, err = c.autoDiscoverContracts(ctx, network)
	case "superchain-registry":
		addresses, err = c.discoverFromSuperchainRegistry(network.Spec.ChainID)
	case "well-known":
		addresses = c.getWellKnownAddresses(network.Spec.NetworkName, network.Spec.ChainID)
	case "manual":
		addresses = c.getManualAddresses(network)
	default:
		return nil, fmt.Errorf("unknown discovery method: %s", discoveryMethod)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to discover contracts: %w", err)
	}

	if addresses == nil {
		return nil, fmt.Errorf("unable to discover contract addresses for network %s (chain ID: %d)",
			network.Spec.NetworkName, network.Spec.ChainID)
	}

	// Set discovery metadata
	addresses.LastDiscoveryTime = metav1.Now()

	// Cache the result
	cacheTimeout := c.cacheTimeout
	if network.Spec.ContractAddresses != nil && network.Spec.ContractAddresses.CacheTimeout > 0 {
		cacheTimeout = network.Spec.ContractAddresses.CacheTimeout
	}

	c.cache[cacheKey] = &CachedNetworkAddresses{
		Addresses: addresses,
		ExpiresAt: time.Now().Add(cacheTimeout),
	}

	return addresses, nil
}

// autoDiscoverContracts attempts multiple discovery strategies automatically
func (c *ContractDiscoveryService) autoDiscoverContracts(
	ctx context.Context,
	network *optimismv1alpha1.OptimismNetwork,
) (*optimismv1alpha1.NetworkContractAddresses, error) {
	// Strategy 1: Query SystemConfig contract if provided
	if network.Spec.ContractAddresses != nil && network.Spec.ContractAddresses.SystemConfigAddr != "" {
		addresses, err := c.discoverFromSystemConfig(
			ctx,
			network.Spec.L1RpcUrl,
			network.Spec.ContractAddresses.SystemConfigAddr,
		)
		if err == nil {
			addresses.DiscoveryMethod = "system-config"
			return addresses, nil
		}
		// Log warning but continue with other methods
	}

	// Strategy 2: Query Superchain Registry as fallback
	registryAddresses, err := c.discoverFromSuperchainRegistry(network.Spec.ChainID)
	if err == nil && registryAddresses != nil {
		registryAddresses.DiscoveryMethod = "superchain-registry"
		return registryAddresses, nil
	}

	// Strategy 3: Fall back to well-known addresses
	wellKnownAddresses := c.getWellKnownAddresses(network.Spec.NetworkName, network.Spec.ChainID)
	if wellKnownAddresses != nil {
		wellKnownAddresses.DiscoveryMethod = "well-known"
		return wellKnownAddresses, nil
	}

	return nil, fmt.Errorf("unable to discover contract addresses using any method")
}

// discoverFromSystemConfig queries the SystemConfig contract for other contract addresses
func (c *ContractDiscoveryService) discoverFromSystemConfig(
	_ context.Context,
	l1RpcUrl,
	systemConfigAddr string,
) (*optimismv1alpha1.NetworkContractAddresses, error) {
	client, err := ethclient.Dial(l1RpcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}
	defer client.Close()

	// For now, return a placeholder implementation
	// In a full implementation, this would query the SystemConfig contract
	// using the contract ABI to get the other contract addresses

	addresses := &optimismv1alpha1.NetworkContractAddresses{
		SystemConfigAddr: systemConfigAddr,
		// TODO: Query actual contract for these addresses
		// L2OutputOracleAddr: systemConfig.L2OutputOracle().Hex(),
		// DisputeGameFactoryAddr: systemConfig.DisputeGameFactory().Hex(),
		// OptimismPortalAddr: systemConfig.OptimismPortal().Hex(),
	}

	return addresses, nil
}

// discoverFromSuperchainRegistry discovers addresses from the Superchain Registry
func (c *ContractDiscoveryService) discoverFromSuperchainRegistry(
	chainID int64,
) (*optimismv1alpha1.NetworkContractAddresses, error) {
	// Placeholder implementation - in a full implementation, this would query
	// the Superchain Registry API or embedded registry data
	return nil, fmt.Errorf("superchain registry discovery not yet implemented")
}

// getWellKnownAddresses returns well-known contract addresses for official networks
func (c *ContractDiscoveryService) getWellKnownAddresses(
	networkName string,
	chainID int64,
) *optimismv1alpha1.NetworkContractAddresses {
	switch {
	case networkName == "op-mainnet" || chainID == 10:
		return &optimismv1alpha1.NetworkContractAddresses{
			L2OutputOracleAddr:         "0xdfe97868233d1aa22e815a266982f2cf17685a27",
			DisputeGameFactoryAddr:     "0xe5965Ab5962eDc7477C8520243A95517CD252fA9",
			OptimismPortalAddr:         "0xbEb5Fc579115071764c7423A4f12eDde41f106Ed",
			SystemConfigAddr:           "0x229047fed2591dbec1eF1118d64F7aF3dB9EB290",
			L1CrossDomainMessengerAddr: "0x25ace71c97B33Cc4729CF772ae268934F7ab5fA1",
			L1StandardBridgeAddr:       "0x99C9fc46f92E8a1c0deC1b1747d010903E884bE1",
			// L2 predeploys
			L2CrossDomainMessengerAddr: "0x4200000000000000000000000000000000000007",
			L2StandardBridgeAddr:       "0x4200000000000000000000000000000000000010",
			L2ToL1MessagePasserAddr:    "0x4200000000000000000000000000000000000016",
			DiscoveryMethod:            "well-known",
		}
	case networkName == "op-sepolia" || chainID == 11155420:
		return &optimismv1alpha1.NetworkContractAddresses{
			L2OutputOracleAddr:         "0x90E9c4f8a994a250F6aEfd61CAFb4F2e895D458F",
			DisputeGameFactoryAddr:     "0x05F9613aDB30026FFd634f38e5C4dFd30a197Fa1",
			OptimismPortalAddr:         "0x16Fc5058F25648194471939df75CF27A2fdC48BC",
			SystemConfigAddr:           "0x034edD2A225f7f429A63E0f1D2084B9E0A93b538",
			L1CrossDomainMessengerAddr: "0x58Cc85b8D04EA49cC6DBd3CbFFd00B4B8D6cb3ef",
			L1StandardBridgeAddr:       "0xFBb0621E0B23b5478B630BD55a5f21f67730B0F1",
			// L2 predeploys (same across all OP Stack chains)
			L2CrossDomainMessengerAddr: "0x4200000000000000000000000000000000000007",
			L2StandardBridgeAddr:       "0x4200000000000000000000000000000000000010",
			L2ToL1MessagePasserAddr:    "0x4200000000000000000000000000000000000016",
			DiscoveryMethod:            "well-known",
		}
	case networkName == "base-mainnet" || chainID == 8453:
		return &optimismv1alpha1.NetworkContractAddresses{
			L2OutputOracleAddr:         "0x56315b90c40730925ec5485cf004d835058518A0",
			DisputeGameFactoryAddr:     "0x43edB88C4B80fDD2AdFF2412A7BebF9dF42cB40e",
			OptimismPortalAddr:         "0x49048044D57e1C92A77f79988d21Fa8fAF74E97e",
			SystemConfigAddr:           "0x73a79Fab69143498Ed3712e519A88a918e1f4072",
			L1CrossDomainMessengerAddr: "0x866E82a600A1414e583f7F13623F1aC5d58b0Afa",
			L1StandardBridgeAddr:       "0x3154Cf16ccdb4C6d922629664174b904d80F2C35",
			// L2 predeploys
			L2CrossDomainMessengerAddr: "0x4200000000000000000000000000000000000007",
			L2StandardBridgeAddr:       "0x4200000000000000000000000000000000000010",
			L2ToL1MessagePasserAddr:    "0x4200000000000000000000000000000000000016",
			DiscoveryMethod:            "well-known",
		}
	default:
		return nil
	}
}

// getManualAddresses returns manually configured addresses
func (c *ContractDiscoveryService) getManualAddresses(
	network *optimismv1alpha1.OptimismNetwork,
) *optimismv1alpha1.NetworkContractAddresses {
	if network.Spec.ContractAddresses == nil {
		return nil
	}

	return &optimismv1alpha1.NetworkContractAddresses{
		SystemConfigAddr:       network.Spec.ContractAddresses.SystemConfigAddr,
		L2OutputOracleAddr:     network.Spec.ContractAddresses.L2OutputOracleAddr,
		DisputeGameFactoryAddr: network.Spec.ContractAddresses.DisputeGameFactoryAddr,
		OptimismPortalAddr:     network.Spec.ContractAddresses.OptimismPortalAddr,
		DiscoveryMethod:        "manual",
	}
}

// isCacheExpired checks if the cached addresses have expired
func (c *ContractDiscoveryService) isCacheExpired(cached *CachedNetworkAddresses) bool {
	return time.Now().After(cached.ExpiresAt)
}

// ClearCache clears the contract address cache
func (c *ContractDiscoveryService) ClearCache() {
	c.cache = make(map[string]*CachedNetworkAddresses)
}
