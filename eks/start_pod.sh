#!/usr/bin/env bash
#
# start_pod.sh — provision/update the EKS cluster, then launch a pod into it.
#
#   1. cdk deploy the cluster stack (and, with --with-workloads, the pipeline pods)
#   2. aws eks update-kubeconfig for the freshly deployed cluster
#   3. kubectl apply a single parameterizable pod (GPU by default)
#
# Run from anywhere; it operates relative to its own directory.

set -euo pipefail

# --- defaults ---------------------------------------------------------------
CLUSTER_STACK="InferenceEksCluster"
WORKLOADS_STACK="InferenceEksWorkloads"

POD_NAME="gpu-smoke-test"
POD_IMAGE="nvidia/cuda:12.4.1-base-ubuntu22.04"
POD_COMMAND="nvidia-smi"
NAMESPACE="default"
USE_GPU=true
DEPLOY_WORKLOADS=false
DRY_RUN=false
REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"

usage() {
  cat <<'EOF'
Usage: start_pod.sh [options]

Deploys the EKS cluster (CDK) and launches a pod into it.

Options:
  --name NAME           Pod name (default: gpu-smoke-test)
  --image IMAGE         Container image (default: nvidia/cuda:12.4.1-base-ubuntu22.04)
  --command "CMD"       Shell command to run in the pod (default: nvidia-smi)
  --namespace NS        Namespace for the pod (default: default)
  --region REGION       AWS region (default: $AWS_REGION or us-east-1)
  --no-gpu              Do not request a GPU / add the GPU toleration
  --with-workloads      Also deploy the pipeline pods stack (InferenceEksWorkloads)
  --dry-run             cdk synth only; print the pod manifest; make no changes
  -h, --help            Show this help

Examples:
  ./start_pod.sh                                   # deploy cluster + GPU smoke test
  ./start_pod.sh --with-workloads                  # also deploy the pipeline pods
  ./start_pod.sh --no-gpu --image busybox \
                 --command "echo hello"            # quick CPU debug pod
  ./start_pod.sh --dry-run                         # preview without deploying
EOF
}

# --- arg parsing ------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)           POD_NAME="$2"; shift 2 ;;
    --image)          POD_IMAGE="$2"; shift 2 ;;
    --command)        POD_COMMAND="$2"; shift 2 ;;
    --namespace)      NAMESPACE="$2"; shift 2 ;;
    --region)         REGION="$2"; shift 2 ;;
    --no-gpu)         USE_GPU=false; shift ;;
    --with-workloads) DEPLOY_WORKLOADS=true; shift ;;
    --dry-run)        DRY_RUN=true; shift ;;
    -h|--help)        usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

# --- prerequisites ----------------------------------------------------------
for tool in cdk aws kubectl; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Error: required tool '$tool' not found on PATH." >&2
    exit 1
  fi
done

cd "$(dirname "$0")"

STACKS=("$CLUSTER_STACK")
if $DEPLOY_WORKLOADS; then
  STACKS+=("$WORKLOADS_STACK")
fi

# --- build the pod manifest -------------------------------------------------
TMP_MANIFEST="$(mktemp)"
trap 'rm -f "$TMP_MANIFEST"' EXIT

{
  echo "apiVersion: v1"
  echo "kind: Pod"
  echo "metadata:"
  echo "  name: ${POD_NAME}"
  echo "  namespace: ${NAMESPACE}"
  echo "spec:"
  echo "  restartPolicy: Never"
  echo "  containers:"
  echo "    - name: ${POD_NAME}"
  echo "      image: ${POD_IMAGE}"
  echo "      command: [\"/bin/sh\", \"-c\"]"
  echo "      args: [\"${POD_COMMAND}\"]"
  if $USE_GPU; then
    echo "      resources:"
    echo "        limits:"
    echo "          nvidia.com/gpu: 1"
  fi
  if $USE_GPU; then
    echo "  nodeSelector:"
    echo "    workload: gpu"
    echo "  tolerations:"
    echo "    - key: nvidia.com/gpu"
    echo "      operator: Exists"
    echo "      effect: NoSchedule"
  fi
} > "$TMP_MANIFEST"

# --- dry run: synth + show manifest, change nothing -------------------------
if $DRY_RUN; then
  echo ">> Dry run: synthesizing stacks (${STACKS[*]})..."
  cdk synth "${STACKS[@]}" >/dev/null
  echo ">> Pod manifest that would be applied:"
  cat "$TMP_MANIFEST"
  exit 0
fi

# --- 1. deploy ---------------------------------------------------------------
echo ">> Deploying stacks: ${STACKS[*]}"
cdk deploy "${STACKS[@]}" --require-approval never

# --- 2. kubeconfig -----------------------------------------------------------
CLUSTER_NAME="$(aws cloudformation describe-stacks \
  --stack-name "$CLUSTER_STACK" \
  --region "$REGION" \
  --query "Stacks[0].Outputs[?OutputKey=='ClusterName'].OutputValue" \
  --output text)"

if [[ -z "$CLUSTER_NAME" || "$CLUSTER_NAME" == "None" ]]; then
  echo "Error: could not read ClusterName output from stack $CLUSTER_STACK." >&2
  exit 1
fi

echo ">> Updating kubeconfig for cluster: $CLUSTER_NAME"
aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$REGION"

# --- 3. launch the pod -------------------------------------------------------
echo ">> Launching pod '$POD_NAME' in namespace '$NAMESPACE'"
kubectl apply -f "$TMP_MANIFEST"

echo ">> Waiting for pod to be ready (timeout 300s)..."
kubectl wait --for=condition=Ready "pod/${POD_NAME}" -n "$NAMESPACE" --timeout=300s || true

echo ">> Pod logs:"
kubectl logs -f "pod/${POD_NAME}" -n "$NAMESPACE" || true
