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

// manifestAccept covers single-arch and multi-arch (manifest list / OCI index)
// images — without the list types the registry 404s HEAD requests for
// multi-platform builds.
var manifestAccept = strings.Join([]string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
}, ", ")

// registryRepoURL returns the registry /v2 base URL and the app's repo path,
// or ok=false when the app pushes to an external/authenticated registry the
// operator doesn't manage.
func registryRepoURL(app *v1.App) (baseURL, repo string, ok bool) {
	if app.Spec.Build.Output != "" || app.Spec.Build.RegistrySecret != "" {
		return "", "", false
	}
	registry := app.Spec.Build.Registry
	if registry == "" {
		registry = defaultBuildRegistry
	}
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimPrefix(registry, "https://")
	return fmt.Sprintf("http://%s/v2", registry), fmt.Sprintf("%s/%s", app.Namespace, app.Name), true
}

// imageExistsInRegistry reports whether the given tag is present in the
// in-cluster registry. checked=false means the answer is unknown (external
// registry, or the registry was unreachable) and callers should fall back to
// status-based logic only.
func imageExistsInRegistry(ctx context.Context, app *v1.App, tag string) (exists, checked bool) {
	baseURL, repo, ok := registryRepoURL(app)
	if !ok {
		return false, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead,
		fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, tag), nil)
	if err != nil {
		return false, false
	}
	req.Header.Set("Accept", manifestAccept)

	httpc := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, true
	case http.StatusNotFound:
		return false, true
	default:
		return false, false
	}
}

// manifestDigest resolves a tag to its manifest digest, or "" if unavailable.
func manifestDigest(ctx context.Context, httpc *http.Client, baseURL, repo, tag string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead,
		fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, tag), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", manifestAccept)
	resp, err := httpc.Do(req)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	return resp.Header.Get("Docker-Content-Digest")
}

// cleanupOldRegistryTags deletes every tag in the app's repo except keepTag.
// Deleting a manifest digest removes ALL tags that reference it, so any tag
// sharing keepTag's digest is left alone — otherwise a cached rebuild that
// produced an identical manifest would take the current image down with it.
func cleanupOldRegistryTags(ctx context.Context, app *v1.App, keepTag string) error {
	baseURL, repo, ok := registryRepoURL(app)
	if !ok {
		return nil
	}

	httpc := &http.Client{Timeout: 10 * time.Second}

	resp, err := httpc.Get(fmt.Sprintf("%s/%s/tags/list", baseURL, repo))
	if err != nil {
		return fmt.Errorf("fetching tags: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var tagsResp struct {
		Tags []string `json:"tags"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return fmt.Errorf("parsing tags: %w", err)
	}

	keepDigest := manifestDigest(ctx, httpc, baseURL, repo, keepTag)
	if keepDigest == "" {
		// Can't identify the current image — deleting anything would be a gamble
		return nil
	}

	for _, tag := range tagsResp.Tags {
		if tag == keepTag {
			continue
		}
		digest := manifestDigest(ctx, httpc, baseURL, repo, tag)
		if digest == "" || digest == keepDigest {
			continue
		}
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete,
			fmt.Sprintf("%s/%s/manifests/%s", baseURL, repo, digest), nil)
		if err != nil {
			continue
		}
		delResp, err := httpc.Do(delReq)
		if err != nil {
			continue
		}
		delResp.Body.Close()
	}
	return nil
}

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
