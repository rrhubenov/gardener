// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/logging/victoria/operator"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VictoriaOperator", func() {
	var (
		ctx context.Context

		managedResourceName = "victoria-operator"
		namespace           = "some-namespace"

		image             = "victoria-operator-image"
		priorityClassName = "priority-class"

		fakeClient client.Client
		deployer   component.DeployWaiter
		values     Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) gomegatypes.GomegaMatcher

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceAccount     *corev1.ServiceAccount
		deployment         *appsv1.Deployment
		vpa                *vpaautoscalingv1.VerticalPodAutoscaler
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "victoria-operator",
				Namespace: namespace,
				Labels:    map[string]string{"app": "victoria-operator"},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "victoria-operator",
				Namespace: namespace,
				Labels:    map[string]string{"app": "victoria-operator"},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To(int32(1)),
				RevisionHistoryLimit: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "victoria-operator"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                              "victoria-operator",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.gardener.cloud/to-runtime-apiserver": "allowed",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  priorityClassName,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "victoria-operator",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--leader-elect",
									"--health-probe-bind-address=:8081",
									"--metrics-bind-address=:8080",
									"--controller.disableReconcileFor=VLAgent,VLCluster,VLogs,VMAgent,VMAlert,VMAlertmanager,VMAlertmanagerConfig,VMAnomaly,VMAuth,VMCluster,VMNodeScrape,VMPodScrape,VMProbe,VMRule,VMScrapeConfig,VMServiceScrape,VMSingle,VMStaticScrape,VMUser",
								},
								Env: []corev1.EnvVar{
									{
										Name:  "WATCH_NAMESPACE",
										Value: "",
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("80m"),
										corev1.ResourceMemory: resource.MustParse("120Mi"),
									},
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("120m"),
										corev1.ResourceMemory: resource.MustParse("520Mi"),
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/health",
											Port: intstr.FromInt32(8081),
										},
									},
									InitialDelaySeconds: 15,
									PeriodSeconds:       20,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromInt32(8081),
										},
									},
									InitialDelaySeconds: 5,
									PeriodSeconds:       10,
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								},
							},
						},
					},
				},
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "victoria-operator",
				Namespace: namespace,
				Labels:    map[string]string{"app": "victoria-operator"},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "victoria-operator",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "victoria-operator",
				Labels: map[string]string{"app": "victoria-operator"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					NonResourceURLs: []string{"/metrics", "/metrics/resources", "/metrics/slis"},
					Verbs:           []string{"get", "watch", "list"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{
						"configmaps", "configmaps/finalizers", "endpoints", "events",
						"persistentvolumeclaims", "persistentvolumeclaims/finalizers",
						"pods/eviction", "secrets", "secrets/finalizers", "services",
						"services/finalizers", "serviceaccounts", "serviceaccounts/finalizers",
					},
					Verbs: []string{"*"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{
						"configmaps/status", "pods", "nodes", "nodes/proxy",
						"nodes/metrics", "namespaces",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{
						"deployments", "deployments/finalizers", "statefulsets",
						"statefulsets/finalizers", "daemonsets", "daemonsets/finalizers",
						"replicasets", "statefulsets/status",
					},
					Verbs: []string{"*"},
				},
				{
					APIGroups: []string{"monitoring.coreos.com"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{
						"clusterrolebindings", "clusterrolebindings/finalizers",
						"clusterroles", "clusterroles/finalizers", "roles", "rolebindings",
					},
					Verbs: []string{"*"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"storageclasses"},
					Verbs:     []string{"list", "get", "watch"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets", "poddisruptionbudgets/finalizers"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"route.openshift.io", "image.openshift.io"},
					Resources: []string{"routers/metrics", "registry/metrics"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingresses", "ingresses/finalizers"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list"},
				},
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"list", "watch", "get"},
				},
				{
					APIGroups: []string{"operator.victoriametrics.com"},
					Resources: []string{
						"vlagents", "vlagents/finalizers", "vlagents/status",
						"vlogs", "vlogs/finalizers", "vlogs/status",
						"vlsingles", "vlsingles/finalizers", "vlsingles/status",
						"vlclusters", "vlclusters/finalizers", "vlclusters/status",
						"vmagents", "vmagents/finalizers", "vmagents/status",
						"vmalertmanagerconfigs", "vmalertmanagerconfigs/finalizers", "vmalertmanagerconfigs/status",
						"vmalertmanagers", "vmalertmanagers/finalizers", "vmalertmanagers/status",
						"vmalerts", "vmalerts/finalizers", "vmalerts/status",
						"vmauths", "vmauths/finalizers", "vmauths/status",
						"vmclusters", "vmclusters/finalizers", "vmclusters/status",
						"vmnodescrapes", "vmnodescrapes/finalizers", "vmnodescrapes/status",
						"vmpodscrapes", "vmpodscrapes/finalizers", "vmpodscrapes/status",
						"vmprobes", "vmprobes/finalizers", "vmprobes/status",
						"vmrules", "vmrules/finalizers", "vmrules/status",
						"vmscrapeconfigs", "vmscrapeconfigs/finalizers", "vmscrapeconfigs/status",
						"vmservicescrapes", "vmservicescrapes/finalizers", "vmservicescrapes/status",
						"vmsingles", "vmsingles/finalizers", "vmsingles/status",
						"vmstaticscrapes", "vmstaticscrapes/finalizers", "vmstaticscrapes/status",
						"vmusers", "vmusers/finalizers", "vmusers/status",
						"vmanomalies", "vmanomalies/finalizers", "vmanomalies/status",
					},
					Verbs: []string{"*"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "victoria-operator",
				Labels: map[string]string{"app": "victoria-operator"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "victoria-operator",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "victoria-operator",
				Namespace: namespace,
			}},
		}
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, values)
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			JustBeforeEach(func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels:          map[string]string{"gardener.cloud/role": "seed-system-component", "care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       ptr.To("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
						KeepObjects: ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResource).To(Equal(expectedRuntimeMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			})

			It("should successfully deploy all resources", func() {
				Expect(managedResource).To(consistOf(
					serviceAccount,
					deployment,
					vpa,
					clusterRole,
					clusterRoleBinding,
				))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the ManagedResource is healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it is already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
		},
	}
)
