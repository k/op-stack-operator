package utils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for OptimismNetwork
const (
	// ConditionConfigurationValid indicates whether the network configuration is valid
	ConditionConfigurationValid = "ConfigurationValid"
	// ConditionContractsDiscovered indicates whether contract addresses have been discovered
	ConditionContractsDiscovered = "ContractsDiscovered"
	// ConditionL1Connected indicates whether L1 RPC endpoint is reachable
	ConditionL1Connected = "L1Connected"
	// ConditionL2Connected indicates whether L2 RPC endpoint is reachable
	ConditionL2Connected = "L2Connected"
)

// Condition reasons
const (
	ReasonValidConfiguration     = "ValidConfiguration"
	ReasonInvalidConfiguration   = "InvalidConfiguration"
	ReasonAddressesResolved      = "AddressesResolved"
	ReasonDiscoveryFailed        = "DiscoveryFailed"
	ReasonRPCEndpointReachable   = "RPCEndpointReachable"
	ReasonRPCEndpointUnreachable = "RPCEndpointUnreachable"
)

// SetCondition sets or updates a condition in the conditions slice
func SetCondition(
	conditions *[]metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	now := metav1.NewTime(time.Now())

	// Find existing condition
	for i, condition := range *conditions {
		if condition.Type == conditionType {
			// Update existing condition
			(*conditions)[i].Status = status
			(*conditions)[i].Reason = reason
			(*conditions)[i].Message = message
			(*conditions)[i].LastTransitionTime = now
			(*conditions)[i].ObservedGeneration = condition.ObservedGeneration // Keep existing generation
			return
		}
	}

	// Add new condition
	*conditions = append(*conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// IsConditionTrue returns true if the condition is present and has status True
func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := GetCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// GetCondition returns the condition with the given type, or nil if not found
func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// SetConditionTrue sets a condition to True status
func SetConditionTrue(conditions *[]metav1.Condition, conditionType, reason, message string) {
	SetCondition(conditions, conditionType, metav1.ConditionTrue, reason, message)
}

// SetConditionFalse sets a condition to False status
func SetConditionFalse(conditions *[]metav1.Condition, conditionType, reason, message string) {
	SetCondition(conditions, conditionType, metav1.ConditionFalse, reason, message)
}

// SetConditionUnknown sets a condition to Unknown status
func SetConditionUnknown(conditions *[]metav1.Condition, conditionType, reason, message string) {
	SetCondition(conditions, conditionType, metav1.ConditionUnknown, reason, message)
}
