package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	v1 "kube-deploy/api/v1alpha1"
)

func deleteRegistryImage(ctx context.Context, app *v1.App) error {
	registry := app.Spec.Build.Registry
	if registry == "" {
		registry = defaultBuildRegistry
	}

	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimPrefix(registry, "https://")

	baseURL := fmt.Sprintf("http://%s/v2", registry)
	repo := fmt.Sprintf("%s/%s", app.Namespace, app.Name)

	c := &http.Client{Timeout: 10 * time.Second}

	resp, err := c.Get(fmt.Sprintf("%s/%s/tags/list", baseURL, repo))
	if err != nil {
		return fmt.Errorf("fetching tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	var tagsResp struct {
		Tags []string `json:"tags"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return fmt.Errorf("parsing tags: %w", err)
	}

	for _, tag := range tagsResp.Tags {
		manifestURL := fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, tag)
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		headResp, err := c.Do(req)
		if err != nil {
			continue
		}
		headResp.Body.Close()

		digest := headResp.Header.Get("Docker-Content-Digest")
		if digest == "" {
			continue
		}

		delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
			fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, digest), nil)
		delResp, err := c.Do(delReq)
		if err != nil {
			continue
		}
		delResp.Body.Close()
	}
	return nil
}
