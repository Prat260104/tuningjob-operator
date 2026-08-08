# TuningJob Examples

This directory contains runnable examples demonstrating how to use the TuningJob operator for hyperparameter tuning.

## Prerequisites

- A running Kubernetes cluster (local kind cluster recommended for testing)
- TuningJob operator deployed to the cluster
- `kubectl` configured to access the cluster

## Quick Start with Kind

```bash
# Create a kind cluster
kind create cluster --name tuning-demo

# Deploy the operator (from repo root)
make deploy IMG=<your-registry>/tuningjob-operator:latest

# OR run locally (for development)
make install
make run
```

## Examples

### 1. Simple Tuning (`simple-tuning.yaml`)

A minimal, self-contained example that runs end-to-end on any cluster. Uses a busybox container to demonstrate the complete workflow without external dependencies.

**What it demonstrates:**
- Parameter injection (numeric ranges and categorical values)
- Trial Job execution
- Result reporting via ConfigMap
- Best trial tracking

**Run it:**
```bash
kubectl apply -f examples/simple-tuning.yaml
```

**Watch trials execute:**
```bash
# Watch all resources for this TuningJob
kubectl get tuningjobs,jobs,configmaps -l tuning.dev/tuning-job=simple-tuning -w

# Check TuningJob status
kubectl get tuningjob simple-tuning -o yaml

# View trial parameters and results
kubectl get jobs -l tuning.dev/tuning-job=simple-tuning -o json | jq -r '.items[] | "\(.metadata.name): \(.metadata.annotations["tuning.dev/parameters"])"'

# Check which trial performed best
kubectl get tuningjob simple-tuning -o jsonpath='{.status.bestTrial}' | jq
```

**Expected output:**
- 5 trial Jobs created sequentially (or in parallel if `parallelism` is set)
- Each Job creates a result ConfigMap with a metric value
- TuningJob status shows the best trial with highest metric

**Cleanup:**
```bash
kubectl delete -f examples/simple-tuning.yaml
kubectl delete jobs,configmaps -l tuning.dev/tuning-job=simple-tuning
```

### 2. ML Training Tuning (`ml-training-tuning.yaml`)

A realistic template for tuning machine learning training jobs. Includes typical ML hyperparameters and shows how to integrate with your own training code.

**Before using:**
1. Replace `your-registry/ml-training:latest` with your actual training image
2. Update dataset volume configuration
3. Adjust resource requests/limits for your workload

**What it demonstrates:**
- Typical ML hyperparameters (learning rate, batch size, optimizer, dropout, weight decay)
- Resource management (CPU, memory, GPU)
- Volume mounts for datasets and model storage
- Parallel trial execution
- Failure handling with `maxFailedTrials`

## How Trial Result Reporting Works

The TuningJob operator uses a **ConfigMap-based protocol** for trials to report their final metric:

### Protocol

1. **Naming convention**: Each trial must create a ConfigMap named:
   ```
   <tuningjob-name>-trial-<trial-index>-result
   ```

2. **ConfigMap structure**:
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: <tuningjob-name>-trial-<trial-index>-result
     namespace: <same-as-tuningjob>
   data:
     metric: "<float-value-as-string>"
   ```

3. **Required RBAC**: Trial Jobs need permission to create ConfigMaps (see ServiceAccount/Role/RoleBinding in examples)

### Implementation Examples

#### Shell Script (busybox/bash)
```bash
# Extract trial index from hostname
TRIAL_INDEX=$(echo $HOSTNAME | grep -o 'trial-[0-9]*' | cut -d'-' -f2)
TUNINGJOB_NAME="my-tuning"
CONFIGMAP_NAME="${TUNINGJOB_NAME}-trial-${TRIAL_INDEX}-result"

# Compute metric
METRIC="0.87"

# Create ConfigMap
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: $CONFIGMAP_NAME
  namespace: default
  labels:
    tuning.dev/tuning-job: $TUNINGJOB_NAME
data:
  metric: "$METRIC"
EOF
```

#### Python
```python
import os
import subprocess
import json

def report_metric(metric_value: float):
    """Report trial metric by creating a Kubernetes ConfigMap."""
    hostname = os.environ['HOSTNAME']
    trial_index = hostname.split('-trial-')[-1].rsplit('-', 1)[0]
    
    tuningjob_name = os.environ.get('TUNINGJOB_NAME', 'my-tuning')
    namespace = os.environ.get('NAMESPACE', 'default')
    configmap_name = f"{tuningjob_name}-trial-{trial_index}-result"
    
    configmap = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": configmap_name,
            "namespace": namespace,
            "labels": {
                "tuning.dev/tuning-job": tuningjob_name
            }
        },
        "data": {
            "metric": str(metric_value)
        }
    }
    
    subprocess.run(
        ["kubectl", "apply", "-f", "-"],
        input=json.dumps(configmap).encode(),
        check=True
    )

# Usage
validation_accuracy = 0.87
report_metric(validation_accuracy)
```

#### Go
```go
package main

import (
    "context"
    "fmt"
    "os"
    "strings"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func reportMetric(metricValue float64) error {
    // Extract trial index from hostname
    hostname := os.Getenv("HOSTNAME")
    parts := strings.Split(hostname, "-trial-")
    if len(parts) != 2 {
        return fmt.Errorf("invalid hostname format: %s", hostname)
    }
    trialIndex := strings.Split(parts[1], "-")[0]
    
    tuningjobName := os.Getenv("TUNINGJOB_NAME")
    if tuningjobName == "" {
        tuningjobName = "my-tuning"
    }
    
    namespace := os.Getenv("NAMESPACE")
    if namespace == "" {
        namespace = "default"
    }
    
    configMapName := fmt.Sprintf("%s-trial-%s-result", tuningjobName, trialIndex)
    
    // Create in-cluster Kubernetes client
    config, err := rest.InClusterConfig()
    if err != nil {
        return err
    }
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return err
    }
    
    // Create ConfigMap
    configMap := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      configMapName,
            Namespace: namespace,
            Labels: map[string]string{
                "tuning.dev/tuning-job": tuningjobName,
            },
        },
        Data: map[string]string{
            "metric": fmt.Sprintf("%f", metricValue),
        },
    }
    
    _, err = clientset.CoreV1().ConfigMaps(namespace).Create(
        context.Background(),
        configMap,
        metav1.CreateOptions{},
    )
    return err
}
```

## Reading Hyperparameters

Hyperparameters are injected as environment variables with the prefix `TUNING_PARAM_`:

```bash
# Example: parameter named "learning_rate" with value "0.01"
echo $TUNING_PARAM_learning_rate  # outputs: 0.01

# Example: parameter named "optimizer" with value "adam"
echo $TUNING_PARAM_optimizer  # outputs: adam
```

### In Your Training Code

**Python:**
```python
import os

learning_rate = float(os.environ['TUNING_PARAM_learning_rate'])
batch_size = int(float(os.environ['TUNING_PARAM_batch_size']))
optimizer = os.environ['TUNING_PARAM_optimizer']
```

**Go:**
```go
import (
    "os"
    "strconv"
)

learningRate, _ := strconv.ParseFloat(os.Getenv("TUNING_PARAM_learning_rate"), 64)
batchSize, _ := strconv.Atoi(os.Getenv("TUNING_PARAM_batch_size"))
optimizer := os.Getenv("TUNING_PARAM_optimizer")
```

## Monitoring and Debugging

### Check TuningJob Status
```bash
# Overall status
kubectl get tuningjob <name> -o yaml

# Quick status summary
kubectl get tuningjob <name> -o jsonpath='{.status}' | jq

# Best trial found so far
kubectl get tuningjob <name> -o jsonpath='{.status.bestTrial}' | jq
```

### List All Trials
```bash
# List trial Jobs
kubectl get jobs -l tuning.dev/tuning-job=<name>

# Show trial parameters
kubectl get jobs -l tuning.dev/tuning-job=<name> -o json | \
  jq -r '.items[] | "\(.metadata.name): \(.metadata.annotations["tuning.dev/parameters"])"'
```

### Check Trial Results
```bash
# List result ConfigMaps
kubectl get configmaps -l tuning.dev/tuning-job=<name>

# View specific result
kubectl get configmap <tuningjob-name>-trial-0-result -o jsonpath='{.data.metric}'
```

### Debug Failed Trials
```bash
# List failed Jobs
kubectl get jobs -l tuning.dev/tuning-job=<name> --field-selector status.successful=0

# Check logs from a failed trial
kubectl logs job/<tuningjob-name>-trial-0
```

### Watch Live Progress
```bash
# Watch all resources
kubectl get tuningjobs,jobs,configmaps -l tuning.dev/tuning-job=<name> -w

# Follow controller logs
kubectl logs -n operator-system deployment/operator-controller-manager -f
```

## Common Issues

### Trial Jobs Don't Create ConfigMaps
- **Cause**: Missing RBAC permissions
- **Solution**: Ensure ServiceAccount, Role, and RoleBinding are created (included in example YAMLs)

### Trials Stay Pending
- **Cause**: Resource constraints or image pull failures
- **Solution**: Check pod status with `kubectl describe pod <pod-name>`

### Best Trial Not Updating
- **Cause**: ConfigMap created with wrong name or missing `metric` key
- **Solution**: Verify ConfigMap naming follows protocol: `<tuningjob>-trial-<index>-result`

### Wrong Parameter Values
- **Cause**: Numeric parameters passed as strings need conversion
- **Solution**: Parse strings to float/int: `float(os.environ['TUNING_PARAM_learning_rate'])`

## Next Steps

- Review the operator's main README for deployment instructions
- Check the API reference for all TuningJob spec fields
- Implement result reporting in your training container
- Start with `simple-tuning.yaml` to verify your setup, then adapt `ml-training-tuning.yaml` for your workload
