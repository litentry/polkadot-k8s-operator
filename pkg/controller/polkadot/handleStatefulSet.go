// Copyright (c) 2020 Swisscom Blockchain AG
// Licensed under MIT License
package polkadot

import (
	"context"
	"github.com/go-logr/logr"
	polkadotv1alpha1 "github.com/swisscom-blockchain/polkadot-k8s-operator/pkg/apis/polkadot/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

//pattern Strategy
type IHandlerStatefulSet interface {
	handleStatefulSetSpecific(r *ReconcilePolkadot, CRInstance *polkadotv1alpha1.Polkadot) (bool, error)
}

type handlerStatefulSetValidator struct {
}
func (h *handlerStatefulSetValidator) handleStatefulSetSpecific(r *ReconcilePolkadot, CRInstance *polkadotv1alpha1.Polkadot) (bool, error){
	return r.handleStatefulSetGeneric(CRInstance, newValidatorStatefulSetForCR(CRInstance))
}

type handlerStatefulSetSentry struct {
}
func (h *handlerStatefulSetSentry) handleStatefulSetSpecific(r *ReconcilePolkadot, CRInstance *polkadotv1alpha1.Polkadot) (bool, error){
	return r.handleStatefulSetGeneric(CRInstance, newSentryStatefulSetForCR(CRInstance))
}

type handlerStatefulSetSentryAndValidator struct {
}
func (h *handlerStatefulSetSentryAndValidator) handleStatefulSetSpecific(r *ReconcilePolkadot, CRInstance *polkadotv1alpha1.Polkadot) (bool, error){
	isForcedRequeue, err := r.handleStatefulSetGeneric(CRInstance, newSentryStatefulSetForCR(CRInstance))
	if isForcedRequeue == ForcedRequeue || err != nil {
		return isForcedRequeue, err
	}
	return r.handleStatefulSetGeneric(CRInstance, newValidatorStatefulSetForCR(CRInstance))
}

type handlerStatefulSetDefault struct {
}
func (h *handlerStatefulSetDefault) handleStatefulSetSpecific(r *ReconcilePolkadot, CRInstance *polkadotv1alpha1.Polkadot) (bool, error){
	return handleSkip()
}

//pattern factory
func getHandlerStatefulSet(CRInstance *polkadotv1alpha1.Polkadot) IHandlerStatefulSet {
	if CRKind(CRInstance.Spec.Kind) == Validator {
		return &handlerStatefulSetValidator{}
	}
	if CRKind(CRInstance.Spec.Kind) == Sentry {
		return &handlerStatefulSetSentry{}
	}
	if CRKind(CRInstance.Spec.Kind) == SentryAndValidator {
		return &handlerStatefulSetSentryAndValidator{}
	}
	return &handlerStatefulSetDefault{}
}

func (r *ReconcilePolkadot) handleStatefulSet(CRInstance *polkadotv1alpha1.Polkadot) (bool, error){

	handler := getHandlerStatefulSet(CRInstance)
	return handler.handleStatefulSetSpecific(r,CRInstance)
}

func (r *ReconcilePolkadot) handleStatefulSetGeneric(CRInstance *polkadotv1alpha1.Polkadot, desiredResource *appsv1.StatefulSet) (bool, error) {

	logger := log.WithValues("Deployment.Namespace", desiredResource.Namespace, "Deployment.Name", desiredResource.Name)

	foundResource, err := r.fetchStatefulSet(desiredResource)
	if err != nil {
		logger.Error(err, "Error on fetch the StatefulSet...")
		return NotForcedRequeue, err
	}
	if foundResource == nil {
		logger.Info("StatefulSet not found...")
		logger.Info("Creating a new StatefulSet...")
		err := r.createStatefulSet(desiredResource, CRInstance, logger)
		if err != nil {
			logger.Error(err, "Error on creating a new StatefulSet...")
			return NotForcedRequeue, err
		}
		logger.Info("Created the new StatefulSet")
		return ForcedRequeue, nil
	}

	if areStatefulSetDifferent(foundResource, desiredResource, logger) {
		logger.Info("Updating the StatefulSet...")
		err := r.updateStatefulSet(desiredResource)
		if err != nil {
			logger.Error(err, "Update StatefulSet Error...")
			return NotForcedRequeue, err
		}
		logger.Info("Updated the StatefulSet...")
	}

	return NotForcedRequeue, nil
}

func (r *ReconcilePolkadot) fetchStatefulSet(obj *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	found := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (r *ReconcilePolkadot) createStatefulSet(statefulSet *appsv1.StatefulSet, CRInstance *polkadotv1alpha1.Polkadot, logger logr.Logger) error {
	err := r.setOwnership(CRInstance, statefulSet)
	if err != nil {
		logger.Error(err, "Error on setting the ownership...")
		return err
	}
	err = r.client.Create(context.TODO(), statefulSet)
	return err
}

func (r *ReconcilePolkadot) updateStatefulSet(obj *appsv1.StatefulSet) error {
	return r.client.Update(context.TODO(), obj)
}

func areStatefulSetDifferent(current *appsv1.StatefulSet, desired *appsv1.StatefulSet, logger logr.Logger) bool {
	result := false

	if isStatefulSetReplicaDifferent(current, desired, logger) {
		result = true
	}
	if isStatefulSetVersionDifferent(current, desired, logger) {
		result = true
	}

	return result
}

func isStatefulSetReplicaDifferent(current *appsv1.StatefulSet, desired *appsv1.StatefulSet, logger logr.Logger) bool {
	size := *desired.Spec.Replicas
	if *current.Spec.Replicas != size {
		logger.Info("Found a replica size mismatch...")
		return true
	}
	return false
}

func isStatefulSetVersionDifferent(current *appsv1.StatefulSet, desired *appsv1.StatefulSet, logger logr.Logger) bool {
	version := desired.ObjectMeta.Labels["version"]
	if current.ObjectMeta.Labels["version"] != version {
		logger.Info("Found a version mismatch...")
		return true
	}
	return false
}
