// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsbotanist

import (
	"errors"
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	awsclient "github.com/gardener/gardener/pkg/client/aws"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws"
	awsv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// IMPORTANT NOTICE
// The following part is only temporarily needed until we have completed the Extensibility epic
// and moved out all provider specifics.
// IMPORTANT NOTICE

var (
	scheme  *runtime.Scheme
	decoder runtime.Decoder
)

func init() {
	scheme = runtime.NewScheme()

	// Workaround for incompatible kubernetes dependencies in gardener/gardener and
	// gardener/gardener-extensions.
	awsSchemeBuilder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(aws.SchemeGroupVersion, &aws.InfrastructureConfig{}, &aws.InfrastructureStatus{})
		return nil
	})
	awsv1alpha1SchemeBuilder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(awsv1alpha1.SchemeGroupVersion, &awsv1alpha1.InfrastructureConfig{}, &awsv1alpha1.InfrastructureStatus{})
		return nil
	})
	schemeBuilder := runtime.NewSchemeBuilder(
		awsv1alpha1SchemeBuilder.AddToScheme,
		awsSchemeBuilder.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))

	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

func infrastructureStatusFromInfrastructure(raw []byte) (*awsv1alpha1.InfrastructureStatus, error) {
	config := &awsv1alpha1.InfrastructureStatus{}
	if _, _, err := decoder.Decode(raw, nil, config); err != nil {
		return nil, err
	}
	return config, nil
}

func findSubnetByPurpose(subnets []awsv1alpha1.Subnet, purpose string) (*awsv1alpha1.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q", purpose)
}

func findSubnetByPurposeAndZone(subnets []awsv1alpha1.Subnet, purpose, zone string) (*awsv1alpha1.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Zone == zone && subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q in zone %q", purpose, zone)
}

func findSecurityGroupByPurpose(securityGroups []awsv1alpha1.SecurityGroup, purpose string) (*awsv1alpha1.SecurityGroup, error) {
	for _, securityGroup := range securityGroups {
		if securityGroup.Purpose == purpose {
			return &securityGroup, nil
		}
	}
	return nil, fmt.Errorf("cannot find security group with purpose %q", purpose)
}

func findInstanceProfileByPurpose(instanceProfiles []awsv1alpha1.InstanceProfile, purpose string) (*awsv1alpha1.InstanceProfile, error) {
	for _, instanceProfile := range instanceProfiles {
		if instanceProfile.Purpose == purpose {
			return &instanceProfile, nil
		}
	}
	return nil, fmt.Errorf("cannot find instance profile with purpose %q", purpose)
}

func findRoleByPurpose(roles []awsv1alpha1.Role, purpose string) (*awsv1alpha1.Role, error) {
	for _, role := range roles {
		if role.Purpose == purpose {
			return &role, nil
		}
	}
	return nil, fmt.Errorf("cannot find role with purpose %q", purpose)
}

// IMPORTANT NOTICE
// The above part is only temporarily needed until we have completed the Extensibility epic
// and moved out all provider specifics.
// IMPORTANT NOTICE

// New takes an operation object <o> and creates a new AWSBotanist object.
func New(o *operation.Operation, purpose string) (*AWSBotanist, error) {
	var (
		cloudProvider gardenv1beta1.CloudProvider
		secret        *corev1.Secret
		region        string
	)

	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
		secret = o.Shoot.Secret
		region = o.Shoot.Info.Spec.Cloud.Region
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
		secret = o.Seed.Secret
		region = o.Seed.Info.Spec.Cloud.Region
	}

	if cloudProvider != gardenv1beta1.CloudProviderAWS {
		return nil, errors.New("cannot instantiate an AWS botanist if neither Shoot nor Seed cluster specifies AWS")
	}

	return &AWSBotanist{
		Operation:         o,
		CloudProviderName: "aws",
		AWSClient:         awsclient.NewClient(string(secret.Data[AccessKeyID]), string(secret.Data[SecretAccessKey]), region),
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *AWSBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
