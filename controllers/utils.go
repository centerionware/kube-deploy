package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }
func boolPtr(b bool) *bool    { return &b }

func buildEnv(env map[string]string) []corev1.EnvVar {
	out := []corev1.EnvVar{}
	for k, v := range env {
		out = append(out, corev1.EnvVar{Name: k, Value: v})
	}
	return out
}

func must(v string) resource.Quantity {
	return resource.MustParse(v)
}

// nullIfEmpty returns nil if the slice is empty, otherwise returns the slice.
// Used for Command/Args so an empty array doesn't override the container's CMD/ENTRYPOINT.
func nullIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}