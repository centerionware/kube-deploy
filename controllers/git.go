package controllers

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/go-git/go-git/v6/storage/memory"

	v1 "kube-deploy/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getLatestCommit(ctx context.Context, c client.Client, app *v1.App) (string, error) {
	auth, err := resolveGitAuth(ctx, c, app)
	if err != nil {
		return "", fmt.Errorf("resolving git auth: %w", err)
	}

	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{app.Spec.Repo},
	})

	refs, err := rem.List(&git.ListOptions{Auth: auth})
	if err != nil {
		return "", fmt.Errorf("listing remote refs for %s: %w", app.Spec.Repo, err)
	}

	var headTarget plumbing.ReferenceName
	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD {
			headTarget = ref.Target()
			break
		}
	}

	for _, ref := range refs {
		if ref.Name() == headTarget {
			return ref.Hash().String(), nil
		}
	}

	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD && !ref.Hash().IsZero() {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("could not resolve HEAD for %s", app.Spec.Repo)
}

func resolveGitAuth(ctx context.Context, c client.Client, app *v1.App) (transport.AuthMethod, error) {
	secretName := app.Spec.Build.GitSecret
	if secretName == "" {
		return nil, nil
	}

	var secret corev1.Secret
	if err := c.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: app.Namespace,
	}, &secret); err != nil {
		return nil, fmt.Errorf("fetching git secret %q: %w", secretName, err)
	}

	if _, ok := secret.Data["ssh-privatekey"]; ok {
		passphrase := ""
		if pp, ok := secret.Data["ssh-passphrase"]; ok {
			passphrase = string(pp)
		}
		keys, err := ssh.NewPublicKeys("git", secret.Data["ssh-privatekey"], passphrase)
		if err != nil {
			return nil, fmt.Errorf("parsing SSH key from secret %q: %w", secretName, err)
		}
		return keys, nil
	}

	if _, ok := secret.Data["password"]; ok {
		username := "git"
		if u, ok := secret.Data["username"]; ok {
			username = string(u)
		}
		return &http.BasicAuth{
			Username: username,
			Password: string(secret.Data["password"]),
		}, nil
	}

	return nil, fmt.Errorf("secret %q has no recognised git auth fields", secretName)
}
