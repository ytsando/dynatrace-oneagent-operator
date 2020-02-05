package utils

import (
	"context"
	"fmt"
	"strings"

	dynatracev1alpha1 "github.com/Dynatrace/dynatrace-oneagent-operator/pkg/apis/dynatrace/v1alpha1"
	"github.com/Dynatrace/dynatrace-oneagent-operator/pkg/dtclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DynatracePaasToken = "paasToken"
	DynatraceApiToken  = "apiToken"
)

// DynatraceClientFunc defines handler func for dynatrace client
type DynatraceClientFunc func(rtc client.Client, instance *dynatracev1alpha1.OneAgent) (dtclient.Client, error)

// BuildDynatraceClient creates a new Dynatrace client using the settings configured on the given instance.
func BuildDynatraceClient(rtc client.Client, instance *dynatracev1alpha1.OneAgent) (dtclient.Client, error) {
	secret := &corev1.Secret{}
	err := rtc.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: instance.Spec.Tokens}, secret)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if err = verifySecret(secret); err != nil {
		return nil, err
	}

	// initialize dynatrace client
	var certificateValidation = dtclient.SkipCertificateValidation(instance.Spec.SkipCertCheck)

	apiToken, err := extractToken(secret, DynatraceApiToken)
	if err != nil {
		return nil, err
	}

	paasToken, err := extractToken(secret, DynatracePaasToken)
	if err != nil {
		return nil, err
	}

	dtc, err := dtclient.NewClient(instance.Spec.ApiUrl, apiToken, paasToken, certificateValidation)

	return dtc, err
}

func extractToken(secret *v1.Secret, key string) (string, error) {
	value, ok := secret.Data[key]
	if !ok {
		err := fmt.Errorf("missing token %s", key)
		return "", err
	}

	return strings.TrimSpace(string(value)), nil
}

func verifySecret(secret *v1.Secret) error {
	for _, token := range []string{DynatracePaasToken, DynatraceApiToken} {
		_, err := extractToken(secret, token)
		if err != nil {
			return fmt.Errorf("invalid secret %s, %s", secret.Name, err)
		}
	}

	return nil
}

// BuildOneAgentLabels returns generic labels based on the name given for a Dynatrace OneAgent.
func BuildOneAgentLabels(name string) map[string]string {
	return map[string]string{
		"dynatrace.com/operator":          "oneagent",
		"oneagent.dynatrace.com/instance": name,
	}
}

// BuildIstioLabels returns labels for Istio objects.
func BuildIstioLabels(name, role string) map[string]string {
	m := BuildOneAgentLabels(name)
	m["oneagent.dynatrace.com/istio-role"] = role
	return m
}

// IsPredefinedLabel returns true if the label is predefined by the Operator.
func IsPredefinedLabel(label string) bool {
	return strings.HasPrefix(label, "dynatrace.com/") || strings.HasPrefix(label, "oneagent.dynatrace.com/")
}

// MergeLabels merges the given labels on their order and returns the result. Any nil argument is ignored.
func MergeLabels(labels ...map[string]string) map[string]string {
	res := map[string]string{}
	for _, m := range labels {
		if m != nil {
			for k, v := range m {
				res[k] = v
			}
		}
	}

	return res
}

// StaticDynatraceClient creates a DynatraceClientFunc always returning c.
func StaticDynatraceClient(c dtclient.Client) DynatraceClientFunc {
	return func(_ client.Client, oa *dynatracev1alpha1.OneAgent) (dtclient.Client, error) {
		return c, nil
	}
}
