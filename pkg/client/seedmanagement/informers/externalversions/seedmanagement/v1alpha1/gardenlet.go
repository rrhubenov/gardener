// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	context "context"
	time "time"

	apisseedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	versioned "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	internalinterfaces "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions/internalinterfaces"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/client/seedmanagement/listers/seedmanagement/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// GardenletInformer provides access to a shared informer and lister for
// Gardenlets.
type GardenletInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() seedmanagementv1alpha1.GardenletLister
}

type gardenletInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewGardenletInformer constructs a new informer for Gardenlet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewGardenletInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredGardenletInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredGardenletInformer constructs a new informer for Gardenlet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredGardenletInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SeedmanagementV1alpha1().Gardenlets(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SeedmanagementV1alpha1().Gardenlets(namespace).Watch(context.TODO(), options)
			},
		},
		&apisseedmanagementv1alpha1.Gardenlet{},
		resyncPeriod,
		indexers,
	)
}

func (f *gardenletInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredGardenletInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *gardenletInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apisseedmanagementv1alpha1.Gardenlet{}, f.defaultInformer)
}

func (f *gardenletInformer) Lister() seedmanagementv1alpha1.GardenletLister {
	return seedmanagementv1alpha1.NewGardenletLister(f.Informer().GetIndexer())
}
