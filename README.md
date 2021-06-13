# gatling-service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-sandbox/gatling-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-sandbox/gatling-service)](https://goreportcard.com/report/github.com/keptn-sandbox/gatling-service)

This implements a gatling-service for Keptn. If you want to learn more about Keptn visit us on [keptn.sh](https://keptn.sh)

## Compatibility Matrix

*Please fill in your versions accordingly*

| Keptn Version    | [gatling-service Docker Image](https://hub.docker.com/r/keptnsandbox/gatling-service/tags) |
|:----------------:|:----------------------------------------:|
|       0.8.3      | tolleiv/gatling-service:0.1.0 |

## Installation

The *gatling-service* can be installed as a part of [Keptn's uniform](https://keptn.sh).

### Deploy in your Kubernetes cluster

To deploy the current version of the *gatling-service* in your Keptn Kubernetes cluster, apply the [`deploy/service.yaml`](deploy/service.yaml) file:

```console
kubectl apply -f deploy/service.yaml
```

This should install the `gatling-service` together with a Keptn `distributor` into the `keptn` namespace, which you can verify using

```console
kubectl -n keptn get deployment gatling-service -o wide
kubectl -n keptn get pods -l run=gatling-service
```

## Usage

The `gatling-service` expects Gatling simulations files in the project specific Keptn repo. It expects those files to be available in the `gatling` subfolder for a service in the stage you want to execute tests. The `gatling` subfolder acts as `GATLING_HOME` and should be structured according to the default structure. It doesn't have to include default Gatling `libs`. These are loaded through the predefined classpath. Also the default `conf` files are copied from the default distribution in case they're missing within the shipyard. 

Here is an example on how to upload the `PerformanceSimulation.scala` simulation file via the Keptn CLI to the dev stage of project sockshop for the service carts:

```
keptn add-resource --project=sockshop --stage=dev --service=carts --resource=PerformanceSimulation.scala --resourceUri=gatling/user-files/simulations/PerformanceSimulation.scala
```

The name of the simulation is derived from the teststrategy name, which is transformed to camel case (e.g. teststrategy: `performance_light` -> `PerformanceLightSimulation`).
It can also be configured through an additional configuration file `gatling.conf.yaml` with a simple testcase to simulation mapping:

```
spec_version: '0.1.0'
workloads:
  - teststrategy: performance
    simulation: BasicSimulation
  - teststrategy: performance_light
    simulation: LightSimulation
```


### Up- or Downgrading

Adapt and use the following command in case you want to up- or downgrade your installed version (specified by the `$VERSION` placeholder):

```console
kubectl -n keptn set image deployment/gatling-service gatling-service=keptnsandbox/gatling-service:$VERSION --record
```

### Uninstall

To delete a deployed *gatling-service*, use the file `deploy/*.yaml` files from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml
```

## Development

Development can be conducted using any GoLang compatible IDE/editor (e.g., Jetbrains GoLand, VSCode with Go plugins).

It is recommended to make use of branches as follows:

* `master` contains the latest potentially unstable version
* `release-*` contains a stable version of the service (e.g., `release-0.1.0` contains version 0.1.0)
* create a new branch for any changes that you are working on, e.g., `feature/my-cool-stuff` or `bug/overflow`
* once ready, create a pull request from that branch back to the `master` branch

When writing code, it is recommended to follow the coding style suggested by the [Golang community](https://github.com/golang/go/wiki/CodeReviewComments).

## License

Please find more information in the [LICENSE](LICENSE) file.
