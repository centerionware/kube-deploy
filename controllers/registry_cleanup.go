package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	v1 "npm-operator/api/v1alpha1"
)

// deleteRegistryImage deletes the image for this app from the registry via the HTTP API.
// This is best-effort — failure is logged but does not block deletion of the NpmApp.
func deleteRegistryImage(ctx context.Context, app *v1.NpmApp) error {
	registry := app.Spec.Build.Registry
	if registry == "" {
		registry = defaultBuildRegistry
	}

	// Strip any scheme prefix — registry API uses http://
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimPrefix(registry, "https://")

	baseURL := fmt.Sprintf("http://%s/v2", registry)
	repo := fmt.Sprintf("%s/%s", app.Namespace, app.Name)

	client := &http.Client{Timeout: 10 * time.Second}

	// Get all tags for this repo
	tagsURL := fmt.Sprintf("%s/%s/tags/list", baseURL, repo)
	resp, err := client.Get(tagsURL)
	if err != nil {
		return fmt.Errorf("fetching tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Nothing to clean up
		return nil
	}

	var tagsResp struct {
		Tags []string `json:"tags"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return fmt.Errorf("parsing tags response: %w", err)
	}

	// Delete each tag's manifest
	for _, tag := range tagsResp.Tags {
		manifestURL := fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, tag)

		// Get the digest
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		headResp, err := client.Do(req)
		if err != nil {
			continue
		}
		headResp.Body.Close()

		digest := headResp.Header.Get("Docker-Content-Digest")
		if digest == "" {
			continue
		}

		// Delete by digest
		deleteURL := fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, digest)
		delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		delResp, err := client.Do(delReq)
		if err != nil {
			continue
		}
		delResp.Body.Close()
	}

	return nil
}
