// +build integration

package integrationtests

import (
	"context"
	"strings"
	"testing"

	dynatracev1alpha1 "github.com/Dynatrace/dynatrace-oneagent-operator/pkg/apis/dynatrace/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileOneAgent_ReconcileOnEmptyEnvironment(t *testing.T) {
	oaName := "oneagent"

	e, err := newTestEnvironment()
	assert.NoError(t, err, "failed to start test environment")

	defer e.Stop()

	e.AddOneAgent(oaName, &dynatracev1alpha1.OneAgentSpec{
		ApiUrl: DefaultTestAPIURL,
		Tokens: "token-test",
	})

	_, err = e.Reconciler.Reconcile(newReconciliationRequest(oaName))
	assert.NoError(t, err, "error reconciling")

	// Check if deamonset has been created and has correct namespace and name.
	var dsList appsv1.DaemonSetList
	err = e.Client.List(context.TODO(), &dsList, client.InNamespace(DefaultTestNamespace))
	assert.NoError(t, err, "failed to get DaemonSet")
	if assert.Equal(t, 1, len(dsList.Items), "incorrect number of DaemonSets") {
		dsActual := &dsList.Items[0]
		assert.Equal(t, DefaultTestNamespace, dsActual.Namespace, "wrong namespace")
		name := dsActual.GetObjectMeta().GetName()
		assert.Truef(t, strings.HasPrefix(name, oaName+"-"), "wrong name: %s", name)
		assert.Equal(t, corev1.DNSClusterFirst, dsActual.Spec.Template.Spec.DNSPolicy, "DNS policy should ClusterFirst by default")
	}

}
