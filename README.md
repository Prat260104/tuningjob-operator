# TuningJob Operator

TuningJob is a Kubernetes operator for hyperparameter optimization, inspired by Kubeflow Katib's OptimizationJob design proposal. It provides a lightweight, stateless approach to hyperparameter tuning by orchestrating trial Jobs with different parameter configurations, tracking their results, and identifying the best-performing configuration. This project was developed as part of the LFX mentorship program to demonstrate the feasibility of a stateless, process-based architecture for hyperparameter search algorithms, serving as a proof-of-concept for potential integration into the Katib ecosystem.

## Architecture

The operator consists of four main components:

**Controller:** A standard Kubernetes controller that reconciles TuningJob custom resources. It creates trial Jobs by cloning a user-provided Job template, injecting hyperparameters as environment variables prefixed with `TUNING_PARAM_`, and tracking trial completion. The controller follows the CronJob pattern for owner references and garbage collection. When a new trial is needed, the controller invokes the suggester binary as a short-lived subprocess to obtain the next parameter configuration.

**Stateless suggester binary:** A standalone executable (`cmd/suggester`) that accepts a JSON request on stdin (containing parameter definitions, optimization goal, and history of past trials) and returns a JSON response on stdout (containing suggested parameter values). It uses random search by default and runs as a separate process for each suggestion request, with no persistent state or background server. This design allows the suggestion algorithm to be swapped, upgraded, or scaled independently of the controller.

**ConfigMap-based result reporting:** Trial workloads report their objective metric by creating a ConfigMap with a specific naming convention and labels. The controller watches for these ConfigMaps, extracts the metric value, and updates the TuningJob status with trial results and the best configuration found so far. This approach requires no sidecar injection or additional infrastructure beyond standard Kubernetes primitives.

**Katib compatibility layer:** A standalone translation package (`internal/katibcompat`) that converts TuningJob's parameter representation into Katib's v1beta1 FeasibleSpace format. This proves that the two APIs can interoperate without requiring a live runtime integration or adding Katib as a module dependency. The translation is lossless for numeric and categorical parameters and includes comprehensive test coverage.

These four components map directly to the LFX proposal's expected outcomes: a working CRD and controller, a pluggable stateless suggester, integration with user workloads via environment variables and ConfigMaps, and demonstrated compatibility with Katib's parameter model.

## Quick Start

First, build the suggester binary and generate manifests:

```bash
make manifests generate build-suggester
```

To run the controller locally against a kind cluster:

```bash
kind create cluster
make install
go run ./cmd/main.go --suggester-binary=$(pwd)/bin/suggester-test
```

In a separate terminal, apply the minimal example:

```bash
kubectl apply -f examples/simple-tuning.yaml
```

Watch for trials to be created and completed:

```bash
kubectl get tuningjobs,jobs,configmaps -l tuning.dev/tuning-job=simple-tuning -w
```

Once trials complete, check the best result:

```bash
kubectl get tuningjob simple-tuning -o jsonpath='{.status.bestTrial}' | jq
```

For a complete walkthrough including how to write your own training container that reports results, see [examples/README.md](examples/README.md).

## Known Limitations

The controller does not yet implement explicit boundary testing for `MaxFailedTrials` (the threshold is checked but not stress-tested), and the `Parallelism` field's throttling behavior is only validated through integration tests rather than isolated unit tests. The `collectPastTrials()` function relies on a real Kubernetes client and is not covered by pure unit tests with mocked responses. The suggester binary's error handling for malformed JSON input is present but lacks an explicit test case, and the controller's behavior when a trial Job is manually deleted outside of normal garbage collection is not explicitly tested. The re-evaluation logic that re-compares all past trials on every reconcile (documented in code comments) has not been stress-tested with large trial counts.

The current suggester implementation uses random search only and ignores past trial history. More sophisticated algorithms like Bayesian optimization or grid search can be added by replacing the suggester binary or extending the `Suggest()` function, but they are not yet implemented.

The Katib compatibility layer (`internal/katibcompat`) is a scoped translation proof that demonstrates parameter compatibility between TuningJob and Katib's v1beta1 Experiment API. It is not a live runtime integration, does not call Katib's suggestion services, and does not import the full Katib module. It exists solely to validate that the two parameter representations can be converted losslessly, providing a foundation for future integration work.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
