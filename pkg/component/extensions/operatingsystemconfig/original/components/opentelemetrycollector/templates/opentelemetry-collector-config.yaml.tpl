extensions:
  file_storage:
    directory: /var/log/otelcol

receivers:
  journald/journal:
    storage: file_storage

  filelog/pods:
    include: [/var/log/pods/kube-system_*/*/*.log]
    storage: file_storage
    include_file_path: true
    operators:
      - type: container
        format: containerd
        add_metadata_from_filepath: true

processors:
  batch:
    timeout: 10s

  filter/drop_localhost_journal:
    logs:
      exclude:
        match_type: strict
        attributes:
          - key: _HOSTNAME
            value: localhost

  filter/keep_units_journal:
    logs:
      include:
        match_type: expr
        expressions:
          - 'attributes["SYSLOG_IDENTIFIER"] == "kernel" or attributes["_SYSTEMD_UNIT"] in ["kubelet.service", "docker.service", "containerd.service", "gardener-node-agent.service"]'

  filter/drop_units_combine:
    logs:
      exclude:
        match_type: expr
        expressions:
          - 'attributes["SYSLOG_IDENTIFIER"] == "kernel" or attributes["_SYSTEMD_UNIT"] in ["kubelet.service", "docker.service", "containerd.service", "gardener-node-agent.service"]'

  attributes/journal_labels:
    actions:
      - key: unit
        from_attribute: SYSLOG_IDENTIFIER
        action: insert
      - key: nodename
        from_attribute: _HOSTNAME
        action: insert

  attributes/combine_labels:
    actions:
      - key: unit
        from_attribute: SYSLOG_IDENTIFIER
        action: insert
      - key: nodename
        from_attribute: _HOSTNAME
        action: insert

  resource/journal:
    attributes:
      - action: insert
        key: job
        value: systemd-journal
      - action: insert
        key: origin
        value: systemd-journal

  resource/combine_journal:
    attributes:
      - action: insert
        key: job
        value: systemd-combine-journal
      - action: insert
        key: origin
        value: systemd-journal

  k8sattributes:
    auth_type: serviceAccount
    api_config:
      kube_config: {{ .APIServerURL }}
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.uid
    extract:
      metadata:
        - k8s.namespace.name
        - k8s.pod.name
        - k8s.pod.labels
        - k8s.pod.node.name
      labels:
        - key: gardener.cloud/role
        - key: origin
        - key: resources.gardener.cloud/managed-by

  filter/pod_drop_empty:
    logs:
      exclude:
        match_type: expr
        expressions:
          - 'resource["k8s.pod.labels.gardener.cloud/role"] == nil and resource["k8s.pod.labels.origin"] == nil and resource["k8s.pod.labels.resources.gardener.cloud/managed-by"] == nil'

  attributes/pod_labels:
    actions:
      - key: resource["k8s.pod.labels.origin"]
        value: "gardener"
        action: upsert
        if: 'resource["k8s.pod.labels.gardener.cloud/role"] != nil or resource["k8s.pod.labels.resources.gardener.cloud/managed-by"] == "gardener"'
      - key: resource["k8s.pod.labels.gardener.cloud/role"]
        value: "default"
        action: upsert
        if: 'resource["k8s.pod.labels.gardener.cloud/role"] == nil'
      - key: nodename
        from_attribute: k8s.pod.node.name
        action: insert
      - key: namespace_name
        from_attribute: k8s.namespace.name
        action: insert
      - key: pod_name
        from_attribute: k8s.pod.name
        action: insert
      - key: container_name
        from_attribute: k8s.container.name
        action: insert
      - key: gardener_cloud_role
        from_attribute: k8s.pod.labels.gardener.cloud/role
        action: insert
      - key: origin
        value: "shoot_system"
        action: upsert

  filter/keep_gardener:
    logs:
      include:
        match_type: strict
        attributes:
          - key: resource["k8s.pod.labels.origin"]
            value: "gardener"

exporters:
  otlphttp:
    endpoint: {{ .clientURL }}
    headers:
      Authorization: "Bearer ${file:{{ .pathAuthToken }}}"
    tls:
      ca_file: {{ .pathCACert }}
      server_name: {{ .valiIngress }}

service:
  extensions: [file_storage]
  pipelines:
    logs/journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/keep_units_journal, attributes/journal_labels, resource/journal, batch]
      exporters: [otlphttp]
    logs/combine_journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/drop_units_combine, attributes/combine_labels, resource/combine_journal, batch]
      exporters: [otlphttp]
    logs/pods:
      receivers: [filelog/pods]
      processors: [k8sattributes, filter/pod_drop_empty, attributes/pod_labels, filter/keep_gardener, batch]
      exporters: [otlphttp]
