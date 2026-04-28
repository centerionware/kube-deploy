package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&NpmApp{},
		&NpmAppList{},
	)

	return nil
}