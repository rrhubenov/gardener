// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ManagedSeedSetLister helps list ManagedSeedSets.
// All objects returned here must be treated as read-only.
type ManagedSeedSetLister interface {
	// List lists all ManagedSeedSets in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*seedmanagementv1alpha1.ManagedSeedSet, err error)
	// ManagedSeedSets returns an object that can list and get ManagedSeedSets.
	ManagedSeedSets(namespace string) ManagedSeedSetNamespaceLister
	ManagedSeedSetListerExpansion
}

// managedSeedSetLister implements the ManagedSeedSetLister interface.
type managedSeedSetLister struct {
	listers.ResourceIndexer[*seedmanagementv1alpha1.ManagedSeedSet]
}

// NewManagedSeedSetLister returns a new ManagedSeedSetLister.
func NewManagedSeedSetLister(indexer cache.Indexer) ManagedSeedSetLister {
	return &managedSeedSetLister{listers.New[*seedmanagementv1alpha1.ManagedSeedSet](indexer, seedmanagementv1alpha1.Resource("managedseedset"))}
}

// ManagedSeedSets returns an object that can list and get ManagedSeedSets.
func (s *managedSeedSetLister) ManagedSeedSets(namespace string) ManagedSeedSetNamespaceLister {
	return managedSeedSetNamespaceLister{listers.NewNamespaced[*seedmanagementv1alpha1.ManagedSeedSet](s.ResourceIndexer, namespace)}
}

// ManagedSeedSetNamespaceLister helps list and get ManagedSeedSets.
// All objects returned here must be treated as read-only.
type ManagedSeedSetNamespaceLister interface {
	// List lists all ManagedSeedSets in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*seedmanagementv1alpha1.ManagedSeedSet, err error)
	// Get retrieves the ManagedSeedSet from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*seedmanagementv1alpha1.ManagedSeedSet, error)
	ManagedSeedSetNamespaceListerExpansion
}

// managedSeedSetNamespaceLister implements the ManagedSeedSetNamespaceLister
// interface.
type managedSeedSetNamespaceLister struct {
	listers.ResourceIndexer[*seedmanagementv1alpha1.ManagedSeedSet]
}
