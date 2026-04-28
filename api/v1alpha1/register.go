package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

var GroupVersion = runtime.NewSchemeGroupVersion("npm.centerionware.app/v1alpha1")

func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&NpmApp{},
		&NpmAppList{},
	)
	return nil
}