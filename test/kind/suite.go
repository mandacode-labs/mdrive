//go:build kind

package kind

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Suite manages a kind cluster for testing.
type Suite struct {
	clusterName string
	kubeconfig  string
	namespace   string
}

// NewSuite creates a new kind test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{
		clusterName: fmt.Sprintf("retrowin-test-%d", time.Now().Unix()),
		namespace:   "retrowin-test",
	}
}

// Start creates a kind cluster and sets up the namespace.
func (s *Suite) Start(t *testing.T) {
	t.Helper()

	// Create kind cluster
	cmd := exec.Command("kind", "create", "cluster", "--name", s.clusterName)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create kind cluster: %s", string(output))

	// Get kubeconfig and write to temp file
	cmd = exec.Command("kind", "get", "kubeconfig", "--name", s.clusterName)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to get kubeconfig: %s", string(output))

	tmpDir := t.TempDir()
	s.kubeconfig = filepath.Join(tmpDir, "kubeconfig")
	err = os.WriteFile(s.kubeconfig, output, 0644)
	require.NoError(t, err, "Failed to write kubeconfig file")

	// Create namespace
	cmd = exec.Command("kubectl", "--kubeconfig", s.kubeconfig, "create", "namespace", s.namespace)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create namespace: %s", string(output))

	t.Logf("Kind cluster %s created successfully", s.clusterName)
}

// Stop deletes the kind cluster.
func (s *Suite) Stop(t *testing.T) {
	t.Helper()

	cmd := exec.Command("kind", "delete", "cluster", "--name", s.clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: failed to delete kind cluster: %s", string(output))
	}
}

// InstallHelmChart installs the retrowin Helm chart.
func (s *Suite) InstallHelmChart(t *testing.T, imageRepository, imageTag string) {
	t.Helper()

	chartPath := "../../deployment/charts/retrowin"

	cmd := exec.Command("helm", "install", "retrowin", chartPath,
		"--kubeconfig", s.kubeconfig,
		"--namespace", s.namespace,
		"--set", fmt.Sprintf("image.repository=%s", imageRepository),
		"--set", fmt.Sprintf("image.tag=%s", imageTag),
		"--set", "replicaCount=1",
		"--timeout", "2m",
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to install Helm chart: %s", string(output))

	t.Log("Helm chart installed successfully")
}

// WaitForDeployment waits for a deployment to be ready.
func (s *Suite) WaitForDeployment(t *testing.T, name string) {
	t.Helper()

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig, "wait",
		"--namespace", s.namespace,
		"--for=condition=available",
		"--timeout=120s",
		fmt.Sprintf("deployment/%s", name),
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Deployment not ready: %s", string(output))
}

// PortForward starts a port-forward and returns the local URL.
func (s *Suite) PortForward(t *testing.T, service string, port int) string {
	t.Helper()

	localPort := getFreePort(t)

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
		"port-forward",
		fmt.Sprintf("service/%s", service),
		fmt.Sprintf("%d:%d", localPort, port),
		"--namespace", s.namespace,
	)

	err := cmd.Start()
	require.NoError(t, err, "Failed to start port-forward")

	// Give port-forward time to establish
	time.Sleep(2 * time.Second)

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	return fmt.Sprintf("http://localhost:%d", localPort)
}

// getFreePort returns a free port number.
func getFreePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to find free port")
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// PodLogs returns the logs of a pod, retrying if the container is not ready.
func (s *Suite) PodLogs(t *testing.T, podName string) string {
	t.Helper()

	var output []byte
	var err error

	// Retry for up to 60s if container is still creating/crashed
	require.Eventually(t, func() bool {
		cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
			"logs", podName,
			"--namespace", s.namespace,
		)
		output, err = cmd.CombinedOutput()
		if err != nil {
			// Retry if container is still creating or crashed
			return false
		}
		return true
	}, 60*time.Second, 2*time.Second, "Failed to get pod logs after retries")

	return string(output)
}

// WaitForDeploymentExists waits for a deployment to exist.
func (s *Suite) WaitForDeploymentExists(t *testing.T, name string) {
	t.Helper()

	require.Eventually(t, func() bool {
		cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
			"get", "deployment", name,
			"--namespace", s.namespace,
		)
		err := cmd.Run()
		return err == nil
	}, 60*time.Second, 2*time.Second, "Deployment %s should exist", name)
}

// GetPodName returns the name of a pod matching the label selector.
// Returns empty string if no pods match.
func (s *Suite) GetPodName(t *testing.T, labelSelector string) string {
	t.Helper()

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
		"get", "pods",
		"--namespace", s.namespace,
		"--selector", labelSelector,
		"--output", "jsonpath={.items[0].metadata.name}",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}

// JobCompleted checks if a job has completed.
func (s *Suite) JobCompleted(t *testing.T, jobName string) bool {
	t.Helper()

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
		"get", "job", jobName,
		"--namespace", s.namespace,
		"--output", "jsonpath={.status.conditions[?(@.type=='Complete')].status}",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return string(output) == "True"
}

// JobExists checks if a job exists.
func (s *Suite) JobExists(t *testing.T, jobName string) bool {
	t.Helper()

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
		"get", "job", jobName,
		"--namespace", s.namespace,
	)
	err := cmd.Run()
	return err == nil
}

// PodIsRunning checks if any pod matching the label selector is in Running state.
func (s *Suite) PodIsRunning(t *testing.T, labelSelector string) bool {
	t.Helper()

	cmd := exec.Command("kubectl", "--kubeconfig", s.kubeconfig,
		"get", "pods",
		"--namespace", s.namespace,
		"--selector", labelSelector,
		"--field-selector", "status.phase=Running",
		"--output", "jsonpath={.items[0].metadata.name}",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return len(output) > 0 && len(string(output)) > 0
}
