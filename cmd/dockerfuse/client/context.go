package client

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"
)

type dockerConfigFile struct {
	CurrentContext string `json:"currentContext"`
}

type dockerEndpointMeta struct {
	Host          string `json:"Host"`
	SkipTLSVerify bool   `json:"SkipTLSVerify"`
}

type dockerContextMeta struct {
	Name      string                        `json:"Name"`
	Endpoints map[string]dockerEndpointMeta `json:"Endpoints"`
}

func getDockerConfigDir() string {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".docker")
	}
	return filepath.Join(home, ".docker")
}

func resolveDockerContextName() string {
	if ctx := os.Getenv("DOCKER_CONTEXT"); ctx != "" {
		return ctx
	}
	// If DOCKER_HOST is explicitly set without DOCKER_CONTEXT, use default
	if os.Getenv("DOCKER_HOST") != "" {
		return "default"
	}
	configPath := filepath.Join(getDockerConfigDir(), "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg dockerConfigFile
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.CurrentContext != "" {
			return cfg.CurrentContext
		}
	}
	return "default"
}

func dockerClientOptsFromContext(ctxName string) ([]client.Opt, error) {
	if ctxName == "" || ctxName == "default" {
		host := os.Getenv("DOCKER_HOST")
		if host != "" {
			helper, err := connhelper.GetConnectionHelper(host)
			if err != nil {
				return nil, err
			}
			if helper != nil {
				httpClient := &http.Client{
					Transport: &http.Transport{
						DialContext: helper.Dialer,
					},
				}
				return []client.Opt{
					client.WithHTTPClient(httpClient),
					client.WithHost(helper.Host),
					client.WithDialContext(helper.Dialer),
				}, nil
			}
		}
		return []client.Opt{client.FromEnv}, nil
	}

	configDir := getDockerConfigDir()
	h := sha256.Sum256([]byte(ctxName))
	ctxID := hex.EncodeToString(h[:])
	metaPath := filepath.Join(configDir, "contexts", "meta", ctxID, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("context %q not found", ctxName)
		}
		return nil, fmt.Errorf("error reading context %q: %w", ctxName, err)
	}

	var meta dockerContextMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("error parsing context %q: %w", ctxName, err)
	}

	dockerEP, ok := meta.Endpoints["docker"]
	if !ok || dockerEP.Host == "" {
		return nil, fmt.Errorf("context %q does not contain a docker endpoint", ctxName)
	}

	helper, err := connhelper.GetConnectionHelper(dockerEP.Host)
	if err != nil {
		return nil, err
	}

	if helper != nil {
		httpClient := &http.Client{
			Transport: &http.Transport{
				DialContext: helper.Dialer,
			},
		}
		return []client.Opt{
			client.WithHTTPClient(httpClient),
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		}, nil
	}

	var opts []client.Opt
	opts = append(opts, client.WithHost(dockerEP.Host))

	tlsDir := filepath.Join(configDir, "contexts", "tls", ctxID, "docker")
	caFile := filepath.Join(tlsDir, "ca.pem")
	certFile := filepath.Join(tlsDir, "cert.pem")
	keyFile := filepath.Join(tlsDir, "key.pem")

	hasCert := false
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			hasCert = true
		}
	}
	hasCA := false
	if _, err := os.Stat(caFile); err == nil {
		hasCA = true
	}

	if hasCert || hasCA {
		opts = append(opts, client.WithTLSClientConfig(caFile, certFile, keyFile))
	}

	if dockerEP.SkipTLSVerify {
		opts = append(opts, func(c *client.Client) error {
			if tr, ok := c.HTTPClient().Transport.(*http.Transport); ok {
				if tr.TLSClientConfig == nil {
					tr.TLSClientConfig = &tls.Config{}
				}
				tr.TLSClientConfig.InsecureSkipVerify = true
			}
			return nil
		})
	}

	return opts, nil
}
