package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	ns, err := k8sutil.GetWatchNamespace()
	if err != nil {
		return err
	}

	m.GetWebhookServer().Register("/inject", &webhook.Admission{Handler: &podInjector{
		namespace: ns,
	}})

	m.GetWebhookServer().Register("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	return nil
}

var logger = log.Log.WithName("oneagent.webhook")

// podAnnotator injects the OneAgent into Pods
type podInjector struct {
	client    client.Client
	apiReader client.Reader
	decoder   *admission.Decoder
	namespace string
}

// podAnnotator adds an annotation to every incoming pods
func (m *podInjector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	logger.Info("injecting into Pod", "name", pod.Name, "generatedName", pod.GenerateName, "namespace", req.Namespace)

	var ns corev1.Namespace
	if err := m.client.Get(ctx, client.ObjectKey{Name: req.Namespace}, &ns); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	inject := ""

	if ns.Labels != nil && ns.Labels["oneagent.dynatrace.com/inject"] != "" {
		inject = ns.Labels["oneagent.dynatrace.com/inject"]
	}

	if pod.Labels != nil && pod.Labels["oneagent.dynatrace.com/inject"] != "" {
		inject = pod.Labels["oneagent.dynatrace.com/inject"]
	}

	if inject == "false" {
		return admission.Patched("")
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	pod.Annotations["oneagent.dynatrace.com/injected"] = "true"

	flavor := "default"
	if v := pod.Annotations["oneagent.dynatrace.com/flavor"]; v != "" {
		flavor = v
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes,
		corev1.Volume{
			Name: "oneagent",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "oneagent-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "dynatrace-oneagent-webhook-config",
				},
			},
		},
		corev1.Volume{
			Name: "oneagent-podinfo",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{Path: "name", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
						{Path: "namespace", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
						{Path: "uid", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.uid"}},
						{Path: "labels", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"}},
						{Path: "annotations", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations"}},
					},
				},
			},
		})

	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:  "install-oneagent",
		Image: "quay.io/lrgar/oneagent-app-only",
		Args:  []string{"bash", "/mnt/config/init.sh"},
		Env: []corev1.EnvVar{
			{
				Name:  "FLAVOR",
				Value: flavor,
			},
			{
				Name: "NODENAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
			{
				Name: "NODEIP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "oneagent",
				MountPath: "/opt/dynatrace/oneagent",
			},
			{
				Name:      "oneagent-config",
				MountPath: "/mnt/config",
			},
		},
	})

	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]

		c.VolumeMounts = append(c.VolumeMounts,
			corev1.VolumeMount{
				Name:      "oneagent",
				MountPath: "/etc/ld.so.preload",
				SubPath:   "ld.so.preload",
			},
			corev1.VolumeMount{
				Name:      "oneagent",
				MountPath: "/opt/dynatrace/oneagent",
			},
			corev1.VolumeMount{
				Name:      "oneagent-podinfo",
				MountPath: "/opt/dynatrace/oneagent/agent/conf/pod",
			})

		c.Env = append(c.Env,
			corev1.EnvVar{
				Name:  "LD_PRELOAD",
				Value: "/opt/dynatrace/oneagent/agent/lib64/liboneagentproc.so",
			},
			corev1.EnvVar{
				Name:  "DT_CONTAINER_NAME",
				Value: c.Name,
			})
	}

	marshaledPod, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectClient injects the client
func (m *podInjector) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// InjectAPIReader injects the API reader
func (m *podInjector) InjectAPIReader(c client.Reader) error {
	m.apiReader = c
	return nil
}

// InjectDecoder injects the decoder
func (m *podInjector) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
