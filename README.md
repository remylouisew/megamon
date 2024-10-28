# megamon
// TODO(user): Add simple overview of use/purpose

## Why MegaMon over Kube State Metrics

MegaMon was created to address shortcoming of using [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics):

* Lack of ability to stop publishing a metric the moment a JobSet completes/fails.
* Difficult/impossible to aggregate metrics across all JobSets if time-line of metrics is not well defined.
* Complexity in querying Node metrics (Node labels are a separate metric)
* Difficult to derive high level metrics (like MTTR) when baseline metrics like Up-ness of jobset containers / nodes require their own complex queries.
* Difficult or impossible to derive metrics like Time-to-provisioning / Time-to-first-up with promql
* Current metrics are very large (they require all Nodes to be published as individual metrics and aggregated later)

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/megamon:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/megamon:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/megamon:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/megamon/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Internal Google Development

```bash
export MEGAMON_IMG=us-central1-docker.pkg.dev/cool-machine-learning/default/megamon:v0.0.2
make docker-build docker-push CONTAINER_TOOL=podman IMG=${MEGAMON_IMG}

gcloud container clusters get-credentials --region us-east5 tpu-provisioner-e2e-us-east5

make deploy IMG=${MEGAMON_IMG}
```# megamon
