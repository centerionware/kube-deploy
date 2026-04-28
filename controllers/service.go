package controllers

import (
	"context"
	"reflect"

	v1 "npm-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureService(ctx context.Context, c client.Client, app v1.NpmApp) error {

	var svc corev1.Service

	err := c.Get(ctx, types.NamespacedName{
		Name:      app.Spec.Service.Name,
		Namespace: app.Namespace,
	}, &svc)

	desiredAnnotations := app.Spec.Service.Annotations

	if errors.IsNotFound(err) {

		svc = corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        app.Spec.Service.Name,
				Namespace:   app.Namespace,
				Annotations: desiredAnnotations,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: app.APIVersion,
						Kind:       app.Kind,
						Name:       app.Name,
						UID:        app.UID,
						Controller: boolPtr(true),
					},
				},
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

	if !reflect.DeepEqual(svc.Annotations, desiredAnnotations) {
		svc.Annotations = desiredAnnotations
		return c.Update(ctx, &svc)
	}

	return nil
}