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

type NpmAppStatus struct {
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	Phase string `json:"phase,omitempty"`
	Commit string `json:"commit,omitempty"`
	Image string `json:"image,omitempty"`
	ImageDigest string `json:"imageDigest,omitempty"`
	LastGoodImage string `json:"lastGoodImage,omitempty"`
	JobName string `json:"jobName,omitempty"`
}