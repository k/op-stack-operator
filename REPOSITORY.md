# OP Stack Kubernetes Operator - Repository Architecture Decision

## Decision Summary

**Decision**: The OP Stack Kubernetes operator will be implemented in a **separate repository** (`op-stack-operator`) rather than integrated into the main Optimism monorepo.

**Date**: January 2025  
**Status**: Approved

## Context

The specification defines a comprehensive Kubernetes operator system with 5 CRDs managing OP Stack components (OptimismNetwork, OpNode, OpBatcher, OpProposer, OpChallenger). We evaluated two primary options for repository structure:

1. **Separate Repository**: Dedicated `op-stack-operator` repository
2. **Integrated Repository**: Implementation within `/Optimism/` monorepo

## Decision Rationale

### Primary Factors

**Target Audience Separation**

- Operator targets DevOps/SRE teams managing infrastructure
- Main repository targets protocol developers building OP Stack components
- Different communities with distinct needs and workflows

**Operational vs Protocol Concerns**

- Kubernetes operators are infrastructure/operational tooling
- Fundamentally different from core protocol components
- Enables focused expertise and governance models

**Distribution and Deployment**

- Operators follow standard Kubernetes distribution patterns (Helm, OLM)
- Simpler for end users to deploy and manage standalone operator
- Cleaner container image builds and registry management

### Supporting Benefits

- **Development Velocity**: Faster CI/CD cycles without full OP Stack build overhead
- **Clear Separation**: Dedicated issue tracking and project management
- **Community Building**: Focused ecosystem around operator functionality
- **Independent Versioning**: Operator releases decoupled from protocol releases

## Implementation Structure

```
op-stack-operator/
├── api/v1alpha1/          # CRD type definitions
├── controllers/           # Controller implementations
├── pkg/
│   ├── discovery/         # Contract address discovery
│   ├── config/           # Configuration management
│   └── utils/            # Shared utilities
├── config/               # Kubernetes manifests
├── helm/                 # Helm charts
├── docs/                 # Operator-specific docs
└── examples/             # Usage examples
```

## Integration Strategy

**Dependency Management**

- Use Go modules to import stable interfaces from ethereum-optimism/optimism
- Reference official OP Stack component container images
- Maintain compatibility matrix with OP Stack releases

**Testing**

- Integration tests using actual component containers
- End-to-end testing with kind clusters
- Compatibility testing across OP Stack versions

**Documentation**

- Clear integration guides with main OP Stack components
- Version compatibility matrices
- Deployment examples for various configurations

## Trade-offs Accepted

**Integration Complexity**: Requires careful dependency management and version synchronization between repositories, but this is manageable with proper Go module usage and clear compatibility contracts.

**Code Duplication**: Some configuration schemas may be duplicated, but this can be minimized by extracting shared interfaces to common libraries.

## Success Criteria

- [ ] Independent development velocity for operator features
- [ ] Clear user experience for operator deployment
- [ ] Maintained compatibility with OP Stack component releases
- [ ] Active community contribution to operator functionality
- [ ] Standard Kubernetes operator distribution patterns

---

This decision enables focused development of the operator while maintaining strong integration with core OP Stack components, ultimately providing better experiences for both developers and operators.
