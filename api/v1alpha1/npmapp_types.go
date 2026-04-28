package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{
	Group:   "npm.centerionware.app",
	Version: "v1alpha1",
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type NpmApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NpmAppSpec   `json:"spec,omitempty"`
	Status NpmAppStatus `json:"status,omitempty"`
}

func (in *NpmApp) DeepCopyObject() runtime.Object {
	out := new(NpmApp)
	*out = *in
	return out
}

// +kubebuilder:object:root=true

type NpmAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NpmApp `json:"items"`
}

func (in *NpmAppList) DeepCopyObject() runtime.Object {
	out := new(NpmAppList)
	*out = *in
	return out
}

type NpmAppSpec struct {
	Repo string `json:"repo"`

	Build NpmBuildSpec `json:"build,omitempty"`
	Run   NpmRunSpec   `json:"run,omitempty"`

	Service NpmServiceSpec `json:"service,omitempty"`

	Env map[string]string `json:"env,omitempty"`
}

type NpmServiceSpec struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

type NpmBuildSpec struct {
	InstallCommand string `json:"installCommand,omitempty"`
	BuildCommand   string `json:"buildCommand,omitempty"`
}

type NpmRunSpec struct {
	Command string `json:"command,omitempty"`
	Port    int    `json:"port,omitempty"`
}

type NpmAppStatus struct {
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	Image              string `json:"image,omitempty"`
	Phase              string `json:"phase,omitempty"`
}