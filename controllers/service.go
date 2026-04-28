package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureService(ctx context.Context, c client.Client, app v1.NpmApp) error {

	var svc corev1.Service

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, &svc)

	if errors.IsNotFound(err) {

		svc = corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": app.Name},
				Ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(app.Spec.Run.Port),
					},
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}

		return c.Create(ctx, &svc)
	}

	return nil
}