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

package hybridbotanist

import (
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/client-go/tools/bootstrap/token/api"
	bootstraptokenutil "k8s.io/client-go/tools/bootstrap/token/util"
)

// generateCloudConfigChart renders the kube-addon-manager configuration for the cloud config user data.
// It will be stored as a Secret and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateCloudConfigChart() (*chartrenderer.RenderedChart, error) {
	var (
		cloudProvider = map[string]interface{}{
			"name": b.ShootCloudBotanist.GetCloudProviderName(),
		}
		serviceNetwork = b.Shoot.GetServiceNetwork()
		userDataConfig = b.ShootCloudBotanist.GenerateCloudConfigUserDataConfig()
	)

	bootstrapTokenSecret, err := b.computeBootstrapToken()
	if err != nil {
		return nil, err
	}
	bootstrapTokenSecretData := bootstrapTokenSecret.Data

	if userDataConfig.ProvisionCloudProviderConfig {
		cloudProviderConfig, err := b.ShootCloudBotanist.GenerateCloudProviderConfig()
		if err != nil {
			return nil, err
		}
		cloudProvider["config"] = cloudProviderConfig
	}

	hyperKube, err := b.ImageVector.FindImage("hyperkube", b.Shoot.Info.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}

	workers := []map[string]interface{}{}
	for _, workerName := range userDataConfig.WorkerNames {
		workers = append(workers, map[string]interface{}{
			"name":       workerName,
			"secretName": b.Shoot.ComputeCloudConfigSecretName(workerName),
		})
	}

	config := map[string]interface{}{
		"cloudProvider": cloudProvider,
		"kubernetes": map[string]interface{}{
			"caCert":     string(b.Secrets["ca"].Data[secrets.DataKeyCertificateCA]),
			"clusterDNS": common.ComputeClusterIP(serviceNetwork, 10),
			// TODO: resolve conformance test issue before changing:
			// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44
			"domain": gardenv1beta1.DefaultDomain,
			"kubelet": map[string]interface{}{
				"bootstrapToken":   bootstraptokenutil.TokenFromIDAndSecret(string(bootstrapTokenSecretData[bootstraptokenapi.BootstrapTokenIDKey]), string(bootstrapTokenSecretData[bootstraptokenapi.BootstrapTokenSecretKey])),
				"parameters":       userDataConfig.KubeletParameters,
				"hostnameOverride": userDataConfig.HostnameOverride,
			},
			"version": b.Shoot.Info.Spec.Kubernetes.Version,
		},
		"images": map[string]interface{}{
			"hyperkube": hyperKube.String(),
		},
		"workers": workers,
	}

	kubeletConfig := b.Shoot.Info.Spec.Kubernetes.Kubelet
	if kubeletConfig != nil {
		config["kubernetes"].(map[string]interface{})["kubelet"].(map[string]interface{})["featureGates"] = kubeletConfig.FeatureGates
	}

	if b.Shoot.CloudProfile.Spec.CABundle != nil {
		config["caBundle"] = *(b.Shoot.CloudProfile.Spec.CABundle)
	}

	return b.ComputeOriginalCloudConfig(config)
}

func (b *HybridBotanist) computeBootstrapToken() (secret *corev1.Secret, err error) {
	var (
		tokenID    = utils.ComputeSHA256Hex([]byte(time.Now().Format("2006-01-02-15")))[:6]
		secretName = bootstraptokenutil.BootstrapTokenSecretName(tokenID)
	)

	secret, err = b.K8sShootClient.GetSecret(metav1.NamespaceSystem, secretName)
	if apierrors.IsNotFound(err) {
		bootstrapTokenSecretKey, err := utils.GenerateRandomStringFromCharset(16, "0123456789abcdefghijklmnopqrstuvwxyz")
		if err != nil {
			return nil, err
		}
		data := map[string][]byte{
			bootstraptokenapi.BootstrapTokenDescriptionKey:      []byte("A bootstrap token generated by Gardener."),
			bootstraptokenapi.BootstrapTokenIDKey:               []byte(tokenID),
			bootstraptokenapi.BootstrapTokenSecretKey:           []byte(bootstrapTokenSecretKey),
			bootstraptokenapi.BootstrapTokenExpirationKey:       []byte(metav1.Now().Add(90 * time.Minute).Format(time.RFC3339)),
			bootstraptokenapi.BootstrapTokenUsageAuthentication: []byte("true"),
			bootstraptokenapi.BootstrapTokenUsageSigningKey:     []byte("true"),
		}
		return b.K8sShootClient.CreateSecret(metav1.NamespaceSystem, secretName, bootstraptokenapi.SecretTypeBootstrapToken, data, true)
	}
	return secret, err
}
