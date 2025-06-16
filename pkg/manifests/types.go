package manifests

import (
	rollupv1alpha1 "github.com/oplabs/opstack-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Config contains configuration data needed for template generation
type Config struct {
	// Genesis contains the genesis.json data
	Genesis string

	// RollupConfig contains the rollup.json data
	RollupConfig string

	// JWTSecret contains the JWT secret for authentication between op-geth and op-node
	JWTSecret string

	// ContractAddresses contains the deployed L1 contract addresses
	ContractAddresses *rollupv1alpha1.ContractAddresses
}

// BaseTemplateData contains common data for all templates
type BaseTemplateData struct {
	Name      string
	Namespace string
	ChainID   int64
}

// OpGethTemplateData contains data for op-geth templates
type OpGethTemplateData struct {
	BaseTemplateData
	Image     string
	Resources corev1.ResourceRequirements
	Storage   rollupv1alpha1.StorageConfig
	Genesis   string
	JWTSecret string
}

// OpNodeTemplateData contains data for op-node templates
type OpNodeTemplateData struct {
	BaseTemplateData
	Image        string
	Resources    corev1.ResourceRequirements
	RollupConfig string
	JWTSecret    string
	L1RpcUrl     string
}

// OpBatcherTemplateData contains data for op-batcher templates
type OpBatcherTemplateData struct {
	BaseTemplateData
	Image                  string
	Resources              corev1.ResourceRequirements
	SignerPrivateKeySecret string
	L1RpcUrl               string
	RollupConfig           string
	ContractAddresses      *rollupv1alpha1.ContractAddresses
}

// OpProposerTemplateData contains data for op-proposer templates
type OpProposerTemplateData struct {
	BaseTemplateData
	Image                  string
	Resources              corev1.ResourceRequirements
	SignerPrivateKeySecret string
	L1RpcUrl               string
	RollupConfig           string
	ContractAddresses      *rollupv1alpha1.ContractAddresses
}
