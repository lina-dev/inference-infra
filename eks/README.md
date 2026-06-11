# eks/ — AWS CDK environment

Provisions the EKS cluster for the inference pipeline and deploys the pipeline
pods, using AWS CDK (Python).

## Stacks

| Stack | Contents |
|---|---|
| `InferenceEksCluster` | Dedicated VPC (2 AZs), EKS 1.31 control plane, a CPU managed nodegroup (`m5.large`) and a GPU managed nodegroup (`g4dn.xlarge`, tainted `nvidia.com/gpu`), and the NVIDIA device plugin. |
| `InferenceEksWorkloads` | The pipeline pods, loaded directly from the existing `infra/k8s` manifests (namespaces → triton/vllm/worker/scheduler/api). |

The workloads stack reuses `infra/k8s/**` as the single source of truth, so the
pod specs never drift from the manifests used elsewhere.

## Prerequisites

- AWS CDK CLI: `npm install -g aws-cdk`
- AWS credentials configured for the target account/region
- `kubectl`, `aws` CLI, and `helm` on PATH
- Python 3.10+

```sh
cd eks
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt

# One-time per account/region:
cdk bootstrap

# Validate locally (no AWS calls beyond credential lookup):
cdk synth
```

## Deploy

```sh
# Cluster only:
cdk deploy InferenceEksCluster

# Cluster + pipeline pods:
cdk deploy --all
```

## start_pod.sh

Orchestrates the common flow: deploy the cluster, point kubeconfig at it, and
launch a pod.

```sh
# Deploy the cluster and run a GPU smoke test (nvidia-smi):
./start_pod.sh

# Also deploy the pipeline pods:
./start_pod.sh --with-workloads

# Launch a CPU debug pod:
./start_pod.sh --no-gpu --image busybox --command "echo hello && sleep 30"

# Preview without changing anything (synths + prints the pod manifest):
./start_pod.sh --dry-run
```

Run `./start_pod.sh --help` for all options.

## Notes

- The deploying IAM principal receives cluster-admin access automatically
  (EKS cluster-creator admin permissions), so `kubectl` works right after deploy.
- The GPU nodegroup scales from 0; the `--gpu` pod path adds a `nvidia.com/gpu`
  request, a `workload: gpu` node selector, and the matching toleration.
- Keep `KUBERNETES_VERSION` in `stacks/cluster_stack.py`, the `KubectlV31Layer`
  import, and the `aws-cdk.lambda-layer-kubectl-v31` pin in `requirements.txt`
  in sync if you upgrade the cluster.
```
