package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeContextMeta(t *testing.T, configDir, ctxName, host string, skipTLS bool) string {
	t.Helper()
	h := sha256.Sum256([]byte(ctxName))
	ctxID := hex.EncodeToString(h[:])
	metaDir := filepath.Join(configDir, "contexts", "meta", ctxID)
	require.NoError(t, os.MkdirAll(metaDir, 0755))

	content := fmt.Sprintf(`{
		"Name": %q,
		"Endpoints": {
			"docker": {
				"Host": %q,
				"SkipTLSVerify": %t
			}
		}
	}`, ctxName, host, skipTLS)

	require.NoError(t, os.WriteFile(filepath.Join(metaDir, "meta.json"), []byte(content), 0644))
	return ctxID
}

func TestResolveDockerContextName(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", tempDir)

	t.Run("defaults to 'default' when nothing set", func(t *testing.T) {
		t.Setenv("DOCKER_CONTEXT", "")
		t.Setenv("DOCKER_HOST", "")
		assert.Equal(t, "default", resolveDockerContextName())
	})

	t.Run("resolves currentContext from config.json", func(t *testing.T) {
		t.Setenv("DOCKER_CONTEXT", "")
		t.Setenv("DOCKER_HOST", "")
		configJSON := `{"currentContext": "my-context"}`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(configJSON), 0644))

		assert.Equal(t, "my-context", resolveDockerContextName())
	})

	t.Run("DOCKER_HOST overrides config.json currentContext", func(t *testing.T) {
		t.Setenv("DOCKER_CONTEXT", "")
		t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		assert.Equal(t, "default", resolveDockerContextName())
	})

	t.Run("DOCKER_CONTEXT overrides everything", func(t *testing.T) {
		t.Setenv("DOCKER_CONTEXT", "explicit-context")
		t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		assert.Equal(t, "explicit-context", resolveDockerContextName())
	})
}

func TestDockerClientOptsFromContext(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")

	t.Run("default context with no DOCKER_HOST returns FromEnv", func(t *testing.T) {
		opts, err := dockerClientOptsFromContext("default")
		require.NoError(t, err)
		assert.Len(t, opts, 1)
	})

	t.Run("default context with ssh DOCKER_HOST returns connhelper opts", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "ssh://user@remote-host")
		opts, err := dockerClientOptsFromContext("default")
		require.NoError(t, err)
		assert.Len(t, opts, 3) // HTTPClient, Host, DialContext
	})

	t.Run("context not found", func(t *testing.T) {
		_, err := dockerClientOptsFromContext("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `context "non-existent" not found`)
	})

	t.Run("context with unix socket", func(t *testing.T) {
		writeContextMeta(t, tempDir, "unix-ctx", "unix:///tmp/test.sock", false)
		opts, err := dockerClientOptsFromContext("unix-ctx")
		require.NoError(t, err)
		assert.Len(t, opts, 1) // WithHost
	})

	t.Run("context with ssh endpoint", func(t *testing.T) {
		writeContextMeta(t, tempDir, "ssh-ctx", "ssh://ubuntu@remote-server", false)
		opts, err := dockerClientOptsFromContext("ssh-ctx")
		require.NoError(t, err)
		assert.Len(t, opts, 3) // HTTPClient, Host, DialContext
	})

	t.Run("context with TLS certificates verifies TLS by default", func(t *testing.T) {
		t.Setenv("DOCKER_TLS_VERIFY", "")
		ctxID := writeContextMeta(t, tempDir, "tls-ctx", "tcp://10.0.0.1:2376", true)
		tlsDir := filepath.Join(tempDir, "contexts", "tls", ctxID, "docker")
		require.NoError(t, os.MkdirAll(tlsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(tlsDir, "ca.pem"), []byte("dummy-ca"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(tlsDir, "cert.pem"), []byte("dummy-cert"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(tlsDir, "key.pem"), []byte("dummy-key"), 0644))

		opts, err := dockerClientOptsFromContext("tls-ctx")
		require.NoError(t, err)
		// TLS is verified by default: WithHost, WithTLSClientConfig (no InsecureSkipVerify)
		assert.Len(t, opts, 2)
	})

	t.Run("context with TLS certificates allows disabling verification via DOCKER_TLS_VERIFY=0", func(t *testing.T) {
		t.Setenv("DOCKER_TLS_VERIFY", "0")
		opts, err := dockerClientOptsFromContext("tls-ctx")
		require.NoError(t, err)
		// WithHost, WithTLSClientConfig, plus InsecureSkipVerify opt
		assert.Len(t, opts, 3)
	})

	t.Run("context without docker endpoint", func(t *testing.T) {
		h := sha256.Sum256([]byte("empty-ctx"))
		ctxID := hex.EncodeToString(h[:])
		metaDir := filepath.Join(tempDir, "contexts", "meta", ctxID)
		require.NoError(t, os.MkdirAll(metaDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(metaDir, "meta.json"), []byte(`{"Name":"empty-ctx","Endpoints":{}}`), 0644))

		_, err := dockerClientOptsFromContext("empty-ctx")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `does not contain a docker endpoint`)
	})
}

func TestNewDockerFuseClientInvalidContext(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "definitely-not-a-real-context")
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	_, err := NewDockerFuseClient("test_container")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `context "definitely-not-a-real-context" not found`)
}
