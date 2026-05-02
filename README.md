# Kube-Deploy

This will include everything required to quickly deploy an npm project from public (maybe private ?) git repositories to kubernetes, create a deployment and a service that's exposed via netbird.


basically a cloudflare worker/vercel highly focused self hosted alternative 

## Motivation 

Kubernetes has a large ecosystem of tooling. Everyone who uses it has used kubectl to apply manifests, helm to install applications, probably kustomize too. Flux and Argocd exist as well, I personally always install flux as one of the first applications when setting up a cluster.

The problem is helm charts need to be maintained, things change rapidly and often the charts lag behind the latest releases, go unmaintained, and it's just a mess.

There's also no native way to build applications where we want to deploy them and an increasing number of projects dont provide a built container for one reason or another. Far to often theres also a built container but only for x86. Arm is getting better support these days but far to often images get built x86 only. With RiscV increasingly coming to market i find the need for a better, easier build system that automates away tedious and fragile build and deploy issues.

With the boom of AI applications there are a lot of projects that dont even provide a Dockerfile anymore.

Thats where kube-deploy comes in. kube-deploy combines simple CRDs with powerful existing tools, such as buildkit and registry, to manage simple installations of apps.

The first target is NPM applications, often the output of these AI systems. It will download the source code from git, generate a dockerfile, use buildkit to build a container, then push that container to a registry. once its in that registry it will generate a deployment and a service. Everything required to build and launch an application using a very targeted and simple to understand CRD.



## current status:

* Partially operational, still in development
* operator runs without error
* Build stage runs, images are pushed to local repository 
* Deployment stage runs
* App's run

## todo:

* add cleanup, watch when crd's are removed and remove any associated jobs and deployments and services (and anything else related in the future) (Preferably also delete from registry)
* Add registry authentication (optional)
* Add namespace to image name (To prevent collissions if the same application is installed to different namespaces)
* Add ingress & gateway api generators (and cleanup)
* improve service definitions to overrides of everything the native kubernetes service definition defines
* Cleanup build jobs when they complete
* Ensure rebuilds on git changes occur, possibly add canary and rollback functions
* add a container launcher to replace the functionality of helm
* integrate autoscaling
* integrated health checks
* add volume creation and mounting
  
## Interesting notes

The registry runs inside of kubernetes, buildkit can access it via DNS resolution within the cluster.

However, the part of kubernetes that actually downloads container images is not inside of kubernetes, so it can't resolve the local registry. Using a NodePort service (instead of clusterip) allows all nodes to access the registry via 'localhost:port' (Even if the registry isn't actually running on that specific node, kubernetes will route it from the node's port to the proper pod)
