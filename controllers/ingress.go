package controllers

import (
	"context"

	v1 "npm-operator/api/v1alpha1"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func EnsureIngress(ctx context.Context, c client.Client, app *v1.NpmApp, port int32) error {
	log := log.FromContext(ctx).WithValues("npmapp", app.Name, "namespace", app.Namespace)

	if app.Spec.Ingress == nil || !app.Spec.Ingress.Enabled {
		// Ingress not requested — delete if it exists
		var existing networkingv1.Ingress
		if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, &existing); err == nil {
			log.Info("removing ingress (disabled)")
			return c.Delete(ctx, &existing)
		}
		return nil
	}

	pathType := networkingv1.PathTypePrefix
	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   app.Namespace,
			Annotations: app.Spec.Ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: app.Spec.Ingress.ClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: app.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: app.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// TLS
	if app.Spec.Ingress.TLSSecret != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{app.Spec.Ingress.Host},
				SecretName: app.Spec.Ingress.TLSSecret,
			},
		}
	}

	var existing networkingv1.Ingress
	err := c.Get(ctx, client.ObjectKeyFromObject(&ing), &existing)
	if errors.IsNotFound(err) {
		log.Info("creating ingress", "host", app.Spec.Ingress.Host)
		return c.Create(ctx, &ing)
	}
	if err != nil {
		return err
	}

	log.Info("updating ingress", "host", app.Spec.Ingress.Host)
	ing.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, &ing)
}
