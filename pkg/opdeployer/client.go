package opdeployer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	rollupv1alpha1 "github.com/oplabs/opstack-operator/api/v1alpha1"
)

// Client handles integration with the op-deployer binary
type Client struct {
	binaryPath string
	workDir    string
}

// NewClient creates a new op-deployer client
func NewClient(binaryPath, workDir string) *Client {
	return &Client{
		binaryPath: binaryPath,
		workDir:    workDir,
	}
}

// DeploymentResult contains the result of an L1 deployment
type DeploymentResult struct {
	// Genesis contains the genesis.json data
	Genesis string

	// RollupConfig contains the rollup.json data
	RollupConfig string

	// StateFile is the path to the deployment state file
	StateFile string

	// ContractAddresses contains the deployed contract addresses
	ContractAddresses *rollupv1alpha1.ContractAddresses
}

// IntentConfig contains the configuration for generating an intent file
type IntentConfig struct {
	L1ChainID          int64
	L2ChainID          int64
	L1RpcUrl           string
	DeployerPrivateKey string
}

// DeployL1Contracts deploys L1 contracts using op-deployer
func (c *Client) DeployL1Contracts(ctx context.Context, config *IntentConfig) (*DeploymentResult, error) {
	// Ensure work directory exists
	if err := os.MkdirAll(c.workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// 1. Generate intent.toml
	_, err := c.generateIntent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate intent: %w", err)
	}

	// 2. Execute op-deployer init
	if err := c.runCommand(ctx, "init",
		"--l1-chain-id", strconv.FormatInt(config.L1ChainID, 10),
		"--l2-chain-ids", strconv.FormatInt(config.L2ChainID, 10),
		"--workdir", c.workDir); err != nil {
		return nil, fmt.Errorf("failed to run op-deployer init: %w", err)
	}

	// 3. Execute op-deployer apply
	if err := c.runCommand(ctx, "apply",
		"--workdir", c.workDir,
		"--l1-rpc-url", config.L1RpcUrl,
		"--private-key", config.DeployerPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to run op-deployer apply: %w", err)
	}

	// 4. Extract configuration and contract addresses
	result := &DeploymentResult{
		StateFile: c.getStateFile(),
	}

	// Extract genesis
	result.Genesis, err = c.extractGenesis(config.L2ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract genesis: %w", err)
	}

	// Extract rollup config
	result.RollupConfig, err = c.extractRollupConfig(config.L2ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract rollup config: %w", err)
	}

	// Extract contract addresses
	result.ContractAddresses, err = c.extractContractAddresses()
	if err != nil {
		return nil, fmt.Errorf("failed to extract contract addresses: %w", err)
	}

	return result, nil
}

// generateIntent generates the intent.toml file for op-deployer
func (c *Client) generateIntent(config *IntentConfig) (string, error) {
	intentContent := fmt.Sprintf(`[L1]
chain_id = %d

[L2]
name = "L2"
chain_id = %d
`, config.L1ChainID, config.L2ChainID)

	intentPath := filepath.Join(c.workDir, "intent.toml")
	if err := os.WriteFile(intentPath, []byte(intentContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write intent file: %w", err)
	}

	return intentPath, nil
}

// runCommand executes an op-deployer command
func (c *Client) runCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	cmd.Dir = c.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// extractGenesis extracts the genesis.json for the given chain ID
func (c *Client) extractGenesis(chainID int64) (string, error) {
	cmd := exec.Command(c.binaryPath, "inspect", "genesis",
		"--workdir", c.workDir,
		strconv.FormatInt(chainID, 10))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to extract genesis: %w", err)
	}

	return string(output), nil
}

// extractRollupConfig extracts the rollup.json for the given chain ID
func (c *Client) extractRollupConfig(chainID int64) (string, error) {
	cmd := exec.Command(c.binaryPath, "inspect", "rollup",
		"--workdir", c.workDir,
		strconv.FormatInt(chainID, 10))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to extract rollup config: %w", err)
	}

	return string(output), nil
}

// extractContractAddresses extracts the deployed contract addresses
func (c *Client) extractContractAddresses() (*rollupv1alpha1.ContractAddresses, error) {
	// This would parse the deployment state file to extract contract addresses
	// For now, return empty addresses - this will be implemented based on
	// the actual op-deployer output format

	stateFile := c.getStateFile()
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return &rollupv1alpha1.ContractAddresses{}, nil
	}

	// TODO: Parse the actual state file format
	// This is a placeholder implementation
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse JSON state file (format TBD based on actual op-deployer output)
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Extract addresses from state (placeholder)
	addresses := &rollupv1alpha1.ContractAddresses{
		// These would be populated from the actual state file
	}

	return addresses, nil
}

// getStateFile returns the path to the deployment state file
func (c *Client) getStateFile() string {
	return filepath.Join(c.workDir, "state.json")
}
