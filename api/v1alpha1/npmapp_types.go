package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------- ROOT TYPES ----------------

type NpmApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NpmAppSpec   `json:"spec,omitempty"`
	Status NpmAppStatus `json:"status,omitempty"`
}

type NpmAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NpmApp `json:"items"`
}

// ---------------- SPEC ----------------

type NpmAppSpec struct {
	Repo string `json:"repo"`

	Env map[string]string `json:"env,omitempty"`

	Build   NpmBuildSpec   `json:"build,omitempty"`
	Run     NpmRunSpec     `json:"run,omitempty"`
	Service NpmServiceSpec `json:"service,omitempty"`
}

type NpmBuildSpec struct {
	BaseImage      string `json:"baseImage,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
	BuildCommand   string `json:"buildCommand,omitempty"`

	Registry RegistrySpec `json:"registry,omitempty"`
}

type RegistrySpec struct {
	URL        string `json:"url,omitempty"`
	Repository string `json:"repository,omitempty"`
	SecretRef  string `json:"secretRef,omitempty"`
}

type NpmRunSpec struct {
	Command []string `json:"command,omitempty"`
	Port    int      `json:"port,omitempty"`
}

type NpmServiceSpec struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ---------------- STATUS ----------------

type NpmAppStatus struct {
	Phase         string `json:"phase,omitempty"`
	Image         string `json:"image,omitempty"`
	Commit        string `json:"commit,omitempty"`
	LastGoodImage string `json:"lastGoodImage,omitempty"`
}

// ---------------- MANUAL SCHEME HOOK ----------------

func AddKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(
		GroupVersion,
		&NpmApp{},
		&NpmAppList{},
	)
	return nil
}