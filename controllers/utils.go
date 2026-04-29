package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func int32Ptr(i int32) *int32 { return &i }

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