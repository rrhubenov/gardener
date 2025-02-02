// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "backupentry"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.BackupEntry{},
			&handler.EnqueueRequestForObject{},
			&predicate.GenerationChangedPredicate{},
			predicateutils.SeedNamePredicate(r.SeedName, gardenerutils.GetBackupEntrySeedNames),
			predicate.NewPredicateFuncs(backupEntryPredicate),
		)).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.BackupBucket{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupBucketToBackupEntry(mgr.GetLogger().WithValues("controller", ControllerName))),
			predicateutils.LastOperationChanged(getBackupBucketLastOperation),
		)).
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&extensionsv1alpha1.BackupEntry{},
			handler.EnqueueRequestsFromMapFunc(r.MapExtensionBackupEntryToCoreBackupEntry(mgr.GetLogger().WithValues("controller", ControllerName))),
			predicateutils.LastOperationChanged(predicateutils.GetExtensionLastOperation),
		)).
		Complete(r)
}

// MapBackupBucketToBackupEntry is a handler.MapFunc for mapping a core.gardener.cloud/v1beta1.BackupBucket to the
// core.gardener.cloud/v1beta1.BackupEntry that references it.
func (r *Reconciler) MapBackupBucketToBackupEntry(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return nil
		}

		backupEntryList := &gardencorev1beta1.BackupEntryList{}
		if err := r.GardenClient.List(ctx, backupEntryList, client.MatchingFields{gardencore.BackupEntryBucketName: backupBucket.Name}); err != nil {
			log.Error(err, "Failed to list backupentries referencing this bucket", "backupBucketName", backupBucket.Name)
			return nil
		}

		return mapper.ObjectListToRequests(backupEntryList, backupEntryPredicate)
	}
}

// MapExtensionBackupEntryToCoreBackupEntry is a handler.MapFunc for mapping an extensions.gardener.cloud/v1alpha1.BackupEntry
// to the owning core.gardener.cloud/v1beta1.BackupEntry.
func (r *Reconciler) MapExtensionBackupEntryToCoreBackupEntry(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetDeletionTimestamp() != nil {
			return nil
		}

		shootTechnicalID, _ := gardenerutils.ExtractShootDetailsFromBackupEntryName(obj.GetName())
		if shootTechnicalID == "" {
			return nil
		}

		shoot, err := extensions.GetShoot(ctx, r.SeedClient, shootTechnicalID)
		if err != nil {
			log.Error(err, "Failed to get shoot from cluster", "shootTechnicalID", shootTechnicalID)
			return nil
		}
		if shoot == nil {
			log.Info("Shoot is missing in cluster resource", "cluster", client.ObjectKey{Name: shootTechnicalID})
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: shoot.Namespace}}}
	}
}

func getBackupBucketLastOperation(obj client.Object) *gardencorev1beta1.LastOperation {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return nil
	}

	return backupBucket.Status.LastOperation
}

// backupEntryPredicate is a predicate which returns true if the core.gardener.cloud/v1beta1.BackupEntry has not yet been successfully migrated or has the `gardener.cloud/operation: restore` annotation.
func backupEntryPredicate(obj client.Object) bool {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return false
	}

	isMigrateSucceeded := backupEntry.Status.LastOperation != nil &&
		backupEntry.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded &&
		backupEntry.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate

	hasRestoreAnnotation := backupEntry.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore

	return !isMigrateSucceeded || hasRestoreAnnotation
}
