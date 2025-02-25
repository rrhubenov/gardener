// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// BackupEntriesGetter has a method to return a BackupEntryInterface.
// A group's client should implement this interface.
type BackupEntriesGetter interface {
	BackupEntries(namespace string) BackupEntryInterface
}

// BackupEntryInterface has methods to work with BackupEntry resources.
type BackupEntryInterface interface {
	Create(ctx context.Context, backupEntry *corev1beta1.BackupEntry, opts v1.CreateOptions) (*corev1beta1.BackupEntry, error)
	Update(ctx context.Context, backupEntry *corev1beta1.BackupEntry, opts v1.UpdateOptions) (*corev1beta1.BackupEntry, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, backupEntry *corev1beta1.BackupEntry, opts v1.UpdateOptions) (*corev1beta1.BackupEntry, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*corev1beta1.BackupEntry, error)
	List(ctx context.Context, opts v1.ListOptions) (*corev1beta1.BackupEntryList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1beta1.BackupEntry, err error)
	BackupEntryExpansion
}

// backupEntries implements BackupEntryInterface
type backupEntries struct {
	*gentype.ClientWithList[*corev1beta1.BackupEntry, *corev1beta1.BackupEntryList]
}

// newBackupEntries returns a BackupEntries
func newBackupEntries(c *CoreV1beta1Client, namespace string) *backupEntries {
	return &backupEntries{
		gentype.NewClientWithList[*corev1beta1.BackupEntry, *corev1beta1.BackupEntryList](
			"backupentries",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *corev1beta1.BackupEntry { return &corev1beta1.BackupEntry{} },
			func() *corev1beta1.BackupEntryList { return &corev1beta1.BackupEntryList{} },
		),
	}
}
