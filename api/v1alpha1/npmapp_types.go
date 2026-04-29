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

// ---------------- REQUIRED K8S INTERFACE ----------------

func (in *NpmApp) DeepCopyObject() runtime.Object {
	out := new(NpmApp)
	*out = *in
	return out
}

func (in *NpmAppList) DeepCopyObject() runtime.Object {
	out := new(NpmAppList)
	*out = *in
	return out
}

// ---------------- SPEC ----------------

type NpmAppSpec struct {
	Repo string `json:"repo"`

	Env map[string]string `json:"env,omitempty"`

	Build NpmBuildSpec `json:"build,omitempty"`

	Run NpmRunSpec `json:"run,omitempty"`

	Service NpmServiceSpec `json:"service,omitempty"`
}

type NpmBuildSpec struct {
	BaseImage   string `json:"baseImage,omitempty"`
	InstallCmd  string `json:"installCmd,omitempty"`
	BuildCmd    string `json:"buildCmd,omitempty"`
	OutputImage string `json:"outputImage,omitempty"`
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
	Phase   string `json:"phase,omitempty"`
	Image   string `json:"image,omitempty"`
	JobName string `json:"jobName,omitempty"`
}