// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

type reconciler struct {
	actuator      Actuator
	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Network resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.Network{} },
		&reconciler{
			actuator:      actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	network := &extensionsv1alpha1.Network{}
	if err := r.client.Get(ctx, request.NamespacedName, network); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, network.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		log.Info("Skipping the reconciliation of Network of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(network.ObjectMeta, network.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, network):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, network, cluster)
	case network.DeletionTimestamp != nil:
		return r.delete(ctx, log, network, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, network, cluster)
	default:
		return r.reconcile(ctx, log, network, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	network *extensionsv1alpha1.Network,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(network, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, network, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, network, operationType, "Reconciling the Network"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of network")
	if err := r.actuator.Reconcile(ctx, log, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, network, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling Network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, network, operationType, "Successfully reconciled Network"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	network *extensionsv1alpha1.Network,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(network, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, network, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, network, gardencorev1beta1.LastOperationTypeRestore, "Restoring the Network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, log, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring Network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, network, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored Network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Network: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	network *extensionsv1alpha1.Network,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(network, FinalizerName) {
		log.Info("Deleting Network causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, log, network, gardencorev1beta1.LastOperationTypeDelete, "Deleting the Network"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of Network")

	var err error
	if cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot) {
		err = r.actuator.ForceDelete(ctx, log, network, cluster)
	} else {
		err = r.actuator.Delete(ctx, log, network, cluster)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting Network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, network, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted Network"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(network, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, network, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	network *extensionsv1alpha1.Network,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if err := r.statusUpdater.Processing(ctx, log, network, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the Network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, log, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating Network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, network, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated Network"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, network); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Network: %+v", err)
	}

	return reconcile.Result{}, nil
}
