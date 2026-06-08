//go:build kind

package kind

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKind_Deployment(t *testing.T) {
	suite := NewSuite(t)
	suite.Start(t)
	t.Cleanup(func() { suite.Stop(t) })

	// Build Docker image and load it into kind
	t.Run("build and load image", func(t *testing.T) {
		cacheRef := os.Getenv("DOCKER_CACHE_REF")
		if cacheRef == "" {
			cacheRef = "registry.mandacode.com/retrowin/cache:main"
		}

		cmd := exec.Command("docker", "buildx", "build",
			"--cache-from", "type=registry,ref="+cacheRef,
			"--load",
			"-f", "../../build/docker/server.Dockerfile",
			"-t", "retrowin:test",
			"../..",
		)
		cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to build Docker image: %s", string(output))

		cmd = exec.Command("kind", "load", "docker-image", "retrowin:test", "--name", suite.clusterName)
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to load image into kind: %s", string(output))
	})

	// Install Helm chart (without --wait, since external deps like DB are not available)
	t.Run("install helm chart", func(t *testing.T) {
		suite.InstallHelmChart(t, "retrowin", "test")
	})

	// Verify deployment exists
	t.Run("deployment exists", func(t *testing.T) {
		suite.WaitForDeploymentExists(t, "retrowin")
	})

	// Verify pods were created
	t.Run("pods were created", func(t *testing.T) {
		podName := suite.GetPodName(t, "app.kubernetes.io/name=retrowin")
		require.NotEmpty(t, podName, "Pod should have been created")
		t.Logf("Pod created: %s", podName)
	})

	// Verify migration job was created
	t.Run("migration job was created", func(t *testing.T) {
		require.Eventually(t, func() bool {
			return suite.JobExists(t, "retrowin-migration-1")
		}, 60*time.Second, 2*time.Second, "Migration job should be created")
	})

	// Wait for pod to reach Running state
	t.Run("pod reaches running state", func(t *testing.T) {
		require.Eventually(t, func() bool {
			return suite.PodIsRunning(t, "app.kubernetes.io/name=retrowin")
		}, 60*time.Second, 2*time.Second, "Pod should reach Running state")
	})

	// Verify config is mounted correctly by checking pod logs
	// Without DB, the server will fail to start, but the config should be read
	t.Run("config is mounted", func(t *testing.T) {
		podName := suite.GetPodName(t, "app.kubernetes.io/name=retrowin")
		require.NotEmpty(t, podName)
		logs := suite.PodLogs(t, podName)
		// The server reads config but fails due to missing DB host.
		// This proves the config was mounted and read.
		assert.Contains(t, logs, "config validation failed", "Config should be mounted and read by the server")
	})

	// Verify service exists and port-forward works
	// The server won't respond without DB, but port-forward should establish
	t.Run("service is accessible via port-forward", func(t *testing.T) {
		baseURL := suite.PortForward(t, "retrowin", 8080)
		require.NotEmpty(t, baseURL, "Port-forward should return a URL")
		t.Logf("Port-forward established at %s", baseURL)
	})
}
