// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetrycollector

import (
	"bytes"
	_ "embed"
	"errors"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	tplNameOpenTelemetryCollector = "config"
	//go:embed templates/opentelemetry-collector-config.yaml.tpl
	tplContentOpenTelemetryCollector string
	tplOpenTelemetryCollector        *template.Template

	shootComponents = []string{"apiserver-proxy", "blackbox-exporter", "calico-node", "calico-typha", "coredns", "egress-filter-applier", "kube-proxy-worker", "kube-proxy-worker", "metrics-server", "network-problem-detector-host", "network-problem-detector-pod", "node-exporter", "node-problem-detector", "vpn-shoot"}
)

func init() {
	tplOpenTelemetryCollector = template.Must(template.
		New(tplNameOpenTelemetryCollector).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentOpenTelemetryCollector))
}

func getOpentelemetryCollectorConfigurationFile(ctx components.Context) (extensionsv1alpha1.File, error) {
	if ctx.ValiIngress == "" {
		return extensionsv1alpha1.File{}, errors.New("opentelemetry-collector ingress url is missing")
	}

	var config bytes.Buffer
	if err := tplOpenTelemetryCollector.Execute(&config, map[string]any{
		"clientURL":       "https://" + ctx.ValiIngress + "/vali/api/v1/push",
		"pathCACert":      PathCACert,
		"valiIngress":     ctx.ValiIngress,
		"pathAuthToken":   PathAuthToken,
		"shootComponents": shootComponents,
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathConfig,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(config.Bytes()),
			},
		},
	}, nil
}

func getOpenTelemetryCollectorCAFile(ctx components.Context) extensionsv1alpha1.File {
	var cABundle []byte
	if ctx.CABundle != nil {
		cABundle = []byte(*ctx.CABundle)
	}

	return extensionsv1alpha1.File{
		Path:        PathCACert,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(cABundle),
			},
		},
	}
}

func getOpenTelemetryCollectorUnit() extensionsv1alpha1.Unit {
	return extensionsv1alpha1.Unit{
		Name:    UnitName,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: ptr.To(`[Unit]
Description=opentelemetry-collector daemon
Documentation=https://github.com/open-telemetry/opentelemetry-collector
[Install]
WantedBy=multi-user.target
[Service]
CPUAccounting=yes
MemoryAccounting=yes
CPUQuota=3%
CPUQuotaPeriodSec=1000ms
MemoryMin=29M
MemoryHigh=400M
MemoryMax=800M
MemorySwapMax=0
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"
ExecStart=` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `/opentelemetry-collector --config=` + PathConfig),
		FilePaths: []string{PathConfig, PathCACert, opentelemetryCollectorBinaryPath},
	}
}
