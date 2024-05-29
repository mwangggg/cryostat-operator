// Copyright The Cryostat Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scorecard

import (
	"context"
	"fmt"
	"net/url"

	operatorv1beta2 "github.com/cryostatio/cryostat-operator/api/v1beta2"
	scapiv1alpha3 "github.com/operator-framework/api/pkg/apis/scorecard/v1alpha3"
	apimanifests "github.com/operator-framework/api/pkg/manifests"
)

const (
	OperatorInstallTestName        string = "operator-install"
	CryostatCRTestName             string = "cryostat-cr"
	CryostatMultiNamespaceTestName string = "cryostat-multi-namespace"
	CryostatRecordingTestName      string = "cryostat-recording"
	CryostatBuiltInTestName        string = "cryostat-discovery"
	CryostatConfigChangeTestName   string = "cryostat-config-change"
	CryostatReportTestName         string = "cryostat-report"
	CryostatAgentTestName          string = "cryostat-agent"
)

// OperatorInstallTest checks that the operator installed correctly
func OperatorInstallTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(OperatorInstallTestName, namespace)

	// Create a new Kubernetes REST client for this test
	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", OperatorInstallTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Poll the deployment until it becomes available or we timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	err = r.waitForDeploymentAvailability(ctx, operatorDeploymentName, namespace)
	if err != nil {
		return r.fail(fmt.Sprintf("operator deployment did not become available: %s", err.Error()))
	}
	return r.TestResult
}

// CryostatCRTest checks that the operator installs Cryostat in response to a Cryostat CR
func CryostatCRTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatCRTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatCRTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Create a default Cryostat CR
	_, err = r.createAndWaitTillCryostatAvailable(r.newCryostatCR())
	if err != nil {
		return r.fail(fmt.Sprintf("%s test failed: %s", CryostatCRTestName, err.Error()))
	}
	return r.TestResult
}

// CryostatMultiNamespaceTest checks that the operator installs multi-namespace Cryostat in response to a multi-namespace Cryostat CR
func CryostatMultiNamespaceTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatMultiNamespaceTestName, namespace)
	r.TargetNamespaces = []string{namespace + "-other"}

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatMultiNamespaceTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	err = r.setupTargetNamespace()
	if err != nil {
		return r.fail(fmt.Sprintf("failed to create target namespaces for %s test: %s", CryostatMultiNamespaceTestName, err.Error()))
	}

	// Create a default ClusterCryostat CR
	_, err = r.createAndWaitTillCryostatAvailable(r.newMultiNamespaceCryostatCR())
	if err != nil {
		return r.fail(fmt.Sprintf("%s test failed: %s", CryostatMultiNamespaceTestName, err.Error()))
	}

	return r.TestResult
}

// CryostatConfigChangeTest checks that the operator redeploys Cryostat in response to a change to Cryostat CR
func CryostatConfigChangeTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatConfigChangeTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatConfigChangeTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Create a default Cryostat CR with default empty dir
	cr := r.newCryostatCR()
	cr.Spec.StorageOptions = &operatorv1beta2.StorageConfiguration{
		EmptyDir: &operatorv1beta2.EmptyDirConfig{
			Enabled: true,
		},
	}

	cr, err = r.createAndWaitTillCryostatAvailable(cr)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to determine application URL: %s", err.Error()))
	}

	// Switch Cryostat CR to PVC for redeployment
	cr, err = r.updateAndWaitTillCryostatAvailable(cr)
	if err != nil {
		return r.fail(fmt.Sprintf("Cryostat redeployment did not become available: %s", err.Error()))
	}
	r.Log += "Cryostat deployment has successfully updated with new spec template\n"

	base, err := url.Parse(cr.Status.ApplicationURL)
	r.Log += fmt.Sprintf("base url: %s\n", base)
	if err != nil {
		return r.fail(fmt.Sprintf("application URL is invalid: %s", err.Error()))
	}

	err = r.waitTillCryostatReady(base)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to reach the application: %s", err.Error()))
	}

	return r.TestResult
}

// TODO add a built in discovery test too
func CryostatRecordingTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatRecordingTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatRecordingTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Create a default Cryostat CR
	cr, err := r.createAndWaitTillCryostatAvailable(r.newCryostatCR())
	if err != nil {
		return r.fail(fmt.Sprintf("failed to determine application URL: %s", err.Error()))
	}
	err = r.StartLogs(cr)
	if err != nil {
		r.Log += fmt.Sprintf("failed to retrieve logs for the application: %s", err.Error())
	}

	base, err := url.Parse(cr.Status.ApplicationURL)
	if err != nil {
		return r.fail(fmt.Sprintf("application URL is invalid: %s", err.Error()))
	}

	err = r.waitTillCryostatReady(base)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to reach the application: %s", err.Error()))
	}

	apiClient := NewCryostatRESTClientset(base)

	// Create a custom target for test
	targetOptions := &Target{
		ConnectUrl: "service:jmx:rmi:///jndi/rmi://localhost:0/jmxrmi",
		Alias:      "customTarget",
	}
	target, err := apiClient.Targets().Create(context.Background(), targetOptions)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to create a target: %s", err.Error()))
	}
	r.Log += fmt.Sprintf("created a custom target: %+v\n", target)

	err = r.recordingFlow(target, apiClient)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to carry out recording actions in %s: %s", CryostatRecordingTestName, err.Error()))
	}

	return r.TestResult
}

/** built in discovery test
func CryostatBuiltInTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatBuiltInTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatBuiltInTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Create a default Cryostat CR
	cr, err := r.createAndWaitTillCryostatAvailable(r.newCryostatCR())
	if err != nil {
		return r.fail(fmt.Sprintf("failed to determine application URL: %s", err.Error()))
	}
	err = r.StartLogs(cr)
	if err != nil {
		r.Log += fmt.Sprintf("failed to retrieve logs for the application: %s", err.Error())
	}

	base, err := url.Parse(cr.Status.ApplicationURL)
	if err != nil {
		return r.fail(fmt.Sprintf("application URL is invalid: %s", err.Error()))
	}

	err = r.waitTillCryostatReady(base)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to reach the application: %s", err.Error()))
	}

	apiClient := NewCryostatRESTClientset(base)

	var target Target
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		targets, err := apiClient.Targets().List(context.Background())
		if err != nil {
			r.Log += fmt.Sprintf("failed to list discovered targets: %s", err.Error())
			return false, err
		}
		if len(targets) == 0 {
			r.Log += "no target is yet discovered\n"
			return false, nil // Try again
		}
		target = targets[0] // Cryostat
		r.Log += fmt.Sprintf("found a target: %+v\n", target)
		return true, nil
	})
	if err != nil {
		return r.fail(fmt.Sprintf("failed to get a target for test: %s", err.Error()))
	}

	err = r.recordingFlow(&target, apiClient)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to carry out recording actions in %s: %s", CryostatBuiltInTestName, err.Error()))
	}

	return r.TestResult
}**/

// CryostatReportTest checks that the operator deploys a report sidecar in response to a Cryostat CR
func CryostatReportTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatReportTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatReportTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	port := int32(10000)
	cr := r.newCryostatCR()
	cr.Spec.ReportOptions = &operatorv1beta2.ReportConfiguration{
		Replicas: 1,
	}
	cr.Spec.ServiceOptions = &operatorv1beta2.ServiceConfigList{
		ReportsConfig: &operatorv1beta2.ReportsServiceConfig{
			HTTPPort: &port,
		},
	}

	// Create a default Cryostat CR
	_, err = r.createAndWaitTillCryostatAvailable(cr)
	if err != nil {
		return r.fail(fmt.Sprintf("%s test failed: %s", CryostatReportTestName, err.Error()))
	}

	// Query health of report sidecar
	err = r.waitTillReportReady(port)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to reach the application: %s", err.Error()))
	}

	err = r.StartLogs(cr)
	if err != nil {
		r.Log += fmt.Sprintf("failed to retrieve logs for the application: %s", err.Error())
	}

	return r.TestResult
}

func CryostatAgentTest(bundle *apimanifests.Bundle, namespace string, openShiftCertManager bool) *scapiv1alpha3.TestResult {
	r := newTestResources(CryostatAgentTestName, namespace)

	err := r.setupCRTestResources(openShiftCertManager)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to set up %s test: %s", CryostatAgentTestName, err.Error()))
	}
	defer r.cleanupAndLogs()

	// Create a default Cryostat CR
	cr, err := r.createAndWaitTillCryostatAvailable(r.newCryostatCR())
	if err != nil {
		return r.fail(fmt.Sprintf("failed to determine application URL: %s", err.Error()))
	}
	err = r.StartLogs(cr)
	if err != nil {
		r.Log += fmt.Sprintf("failed to retrieve logs for the application: %s", err.Error())
	}

	base, err := url.Parse(cr.Status.ApplicationURL)
	if err != nil {
		return r.fail(fmt.Sprintf("application URL is invalid: %s", err.Error()))
	}

	err = r.waitTillCryostatReady(base)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to reach the application: %s", err.Error()))
	}

	svc := r.newSampleService()
	_, err = r.applyAndCreateSampleApplication(r.newSampleApp(), svc)
	if err != nil {
		return r.fail(fmt.Sprintf("application failed to be deployed: %s", err.Error()))
	}

	apiClient := NewCryostatRESTClientset(base)

	// Look for sample app target
	target, err := r.getSampleAppTarget(apiClient)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to register sample app: %s", err.Error()))
	}
	r.Log += fmt.Sprintf("created a quarkus-test-agent target: %+v\n", target)

	err = r.recordingFlow(target, apiClient)
	if err != nil {
		return r.fail(fmt.Sprintf("failed to carry out recording actions in %s: %s", CryostatRecordingTestName, err.Error()))
	}

	return r.TestResult
}
