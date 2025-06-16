package manifests

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	rollupv1alpha1 "github.com/oplabs/opstack-operator/api/v1alpha1"
)

//go:embed templates/**/*.yaml.tmpl
var templateFS embed.FS

// Generator handles the generation of Kubernetes manifests from templates
type Generator struct {
	templates map[string]*template.Template
}

// NewGenerator creates a new manifest generator
func NewGenerator() (*Generator, error) {
	g := &Generator{
		templates: make(map[string]*template.Template),
	}

	// Load templates for each component
	components := []string{"op-geth", "op-node", "op-batcher", "op-proposer"}
	for _, comp := range components {
		pattern := fmt.Sprintf("templates/%s/*.yaml.tmpl", comp)
		tmpl, err := template.ParseFS(templateFS, pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse templates for %s: %w", comp, err)
		}
		g.templates[comp] = tmpl
	}

	return g, nil
}

// GenerateManifests generates all Kubernetes manifests for the given OPChain
func (g *Generator) GenerateManifests(opchain *rollupv1alpha1.OPChain, config *Config) ([]client.Object, error) {
	var manifests []client.Object

	// Generate op-geth manifests
	if opchain.Spec.Components.Geth.Enabled {
		gethManifests, err := g.generateOpGeth(opchain, config)
		if err != nil {
			return nil, fmt.Errorf("failed to generate op-geth manifests: %w", err)
		}
		manifests = append(manifests, gethManifests...)
	}

	// Generate op-node manifests
	if opchain.Spec.Components.Node.Enabled {
		nodeManifests, err := g.generateOpNode(opchain, config)
		if err != nil {
			return nil, fmt.Errorf("failed to generate op-node manifests: %w", err)
		}
		manifests = append(manifests, nodeManifests...)
	}

	// Generate op-batcher manifests
	if opchain.Spec.Components.Batcher.Enabled {
		batcherManifests, err := g.generateOpBatcher(opchain, config)
		if err != nil {
			return nil, fmt.Errorf("failed to generate op-batcher manifests: %w", err)
		}
		manifests = append(manifests, batcherManifests...)
	}

	// Generate op-proposer manifests
	if opchain.Spec.Components.Proposer.Enabled {
		proposerManifests, err := g.generateOpProposer(opchain, config)
		if err != nil {
			return nil, fmt.Errorf("failed to generate op-proposer manifests: %w", err)
		}
		manifests = append(manifests, proposerManifests...)
	}

	return manifests, nil
}

// generateOpGeth generates manifests for op-geth
func (g *Generator) generateOpGeth(opchain *rollupv1alpha1.OPChain, config *Config) ([]client.Object, error) {
	data := &OpGethTemplateData{
		BaseTemplateData: BaseTemplateData{
			Name:      opchain.Name,
			Namespace: opchain.Namespace,
			ChainID:   opchain.Spec.ChainID,
		},
		Image:     opchain.Spec.Components.Geth.Image,
		Resources: opchain.Spec.Components.Geth.Resources,
		Storage:   opchain.Spec.Components.Geth.Storage,
		Genesis:   config.Genesis,
		JWTSecret: config.JWTSecret,
	}

	return g.executeTemplates("op-geth", data)
}

// generateOpNode generates manifests for op-node
func (g *Generator) generateOpNode(opchain *rollupv1alpha1.OPChain, config *Config) ([]client.Object, error) {
	data := &OpNodeTemplateData{
		BaseTemplateData: BaseTemplateData{
			Name:      opchain.Name,
			Namespace: opchain.Namespace,
			ChainID:   opchain.Spec.ChainID,
		},
		Image:        opchain.Spec.Components.Node.Image,
		Resources:    opchain.Spec.Components.Node.Resources,
		RollupConfig: config.RollupConfig,
		JWTSecret:    config.JWTSecret,
		L1RpcUrl:     opchain.Spec.L1.RpcUrl,
	}

	return g.executeTemplates("op-node", data)
}

// generateOpBatcher generates manifests for op-batcher
func (g *Generator) generateOpBatcher(opchain *rollupv1alpha1.OPChain, config *Config) ([]client.Object, error) {
	data := &OpBatcherTemplateData{
		BaseTemplateData: BaseTemplateData{
			Name:      opchain.Name,
			Namespace: opchain.Namespace,
			ChainID:   opchain.Spec.ChainID,
		},
		Image:                  opchain.Spec.Components.Batcher.Image,
		Resources:              opchain.Spec.Components.Batcher.Resources,
		SignerPrivateKeySecret: opchain.Spec.Components.Batcher.SignerPrivateKeySecret,
		L1RpcUrl:               opchain.Spec.L1.RpcUrl,
		RollupConfig:           config.RollupConfig,
		ContractAddresses:      config.ContractAddresses,
	}

	return g.executeTemplates("op-batcher", data)
}

// generateOpProposer generates manifests for op-proposer
func (g *Generator) generateOpProposer(opchain *rollupv1alpha1.OPChain, config *Config) ([]client.Object, error) {
	data := &OpProposerTemplateData{
		BaseTemplateData: BaseTemplateData{
			Name:      opchain.Name,
			Namespace: opchain.Namespace,
			ChainID:   opchain.Spec.ChainID,
		},
		Image:                  opchain.Spec.Components.Proposer.Image,
		Resources:              opchain.Spec.Components.Proposer.Resources,
		SignerPrivateKeySecret: opchain.Spec.Components.Proposer.SignerPrivateKeySecret,
		L1RpcUrl:               opchain.Spec.L1.RpcUrl,
		RollupConfig:           config.RollupConfig,
		ContractAddresses:      config.ContractAddresses,
	}

	return g.executeTemplates("op-proposer", data)
}

// executeTemplates executes all templates for a component and returns Kubernetes objects
func (g *Generator) executeTemplates(component string, data interface{}) ([]client.Object, error) {
	tmpl, exists := g.templates[component]
	if !exists {
		return nil, fmt.Errorf("no templates found for component %s", component)
	}

	var manifests []client.Object
	for _, t := range tmpl.Templates() {
		if t.Name() == "" {
			continue
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute template %s: %w", t.Name(), err)
		}

		// Parse YAML into unstructured object first to detect kind
		var unstructuredObj unstructured.Unstructured
		if err := yaml.Unmarshal(buf.Bytes(), &unstructuredObj); err != nil {
			return nil, fmt.Errorf("failed to parse YAML from template %s: %w", t.Name(), err)
		}

		// Create the appropriate typed object based on Kind
		obj, err := g.createTypedObject(&unstructuredObj)
		if err != nil {
			return nil, fmt.Errorf("failed to create typed object from template %s: %w", t.Name(), err)
		}

		manifests = append(manifests, obj)
	}

	return manifests, nil
}

// createTypedObject creates a properly typed Kubernetes object from unstructured data
func (g *Generator) createTypedObject(unstructuredObj *unstructured.Unstructured) (client.Object, error) {
	kind := unstructuredObj.GetKind()

	var obj runtime.Object
	switch kind {
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "Service":
		obj = &corev1.Service{}
	case "ConfigMap":
		obj = &corev1.ConfigMap{}
	case "Secret":
		obj = &corev1.Secret{}
	case "PersistentVolumeClaim":
		obj = &corev1.PersistentVolumeClaim{}
	default:
		return nil, fmt.Errorf("unsupported Kubernetes object kind: %s", kind)
	}

	// Convert unstructured to typed object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, obj); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to %s: %w", kind, err)
	}

	return obj.(client.Object), nil
}
