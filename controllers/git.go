package controllers

import (
	"fmt"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/storage/memory"
)

func getLatestCommit(repo string) (string, error) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repo},
	})

	refs, err := rem.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing remote refs for %s: %w", repo, err)
	}

	// First pass: find what HEAD points to (it's a symref e.g. refs/heads/main)
	var headTarget plumbing.ReferenceName
	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD {
			headTarget = ref.Target()
			break
		}
	}

	// Second pass: resolve the target branch to its actual commit hash
	for _, ref := range refs {
		if ref.Name() == headTarget {
			return ref.Hash().String(), nil
		}
	}

	// Fallback: if HEAD is a direct hash ref (not symref), use it directly
	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD && !ref.Hash().IsZero() {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("could not resolve HEAD for %s", repo)
}
