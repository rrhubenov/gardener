// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

// ShootApplyConfiguration represents an declarative configuration of the Shoot type for use
// with apply.
type ShootApplyConfiguration struct {
	Name *string `json:"name,omitempty"`
}

// ShootApplyConfiguration constructs an declarative configuration of the Shoot type for use with
// apply.
func Shoot() *ShootApplyConfiguration {
	return &ShootApplyConfiguration{}
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *ShootApplyConfiguration) WithName(value string) *ShootApplyConfiguration {
	b.Name = &value
	return b
}
