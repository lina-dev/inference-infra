"""Workloads stack: applies the existing infra/k8s manifests as pods on the cluster.

Single source of truth: rather than redefining the pipeline pods here, this stack
reads the YAML under ``infra/k8s`` and registers each document with the cluster via
``add_manifest``. Namespaces are applied first; every other group depends on them.
"""

from pathlib import Path

import yaml
from aws_cdk import Stack
from aws_cdk import aws_eks as eks
from constructs import Construct

# Repo layout: this file is eks/stacks/workloads_stack.py, so the repo root is two
# levels up, and the manifests live under infra/k8s.
REPO_ROOT = Path(__file__).resolve().parents[2]
K8S_DIR = REPO_ROOT / "infra" / "k8s"

# Manifest groups in apply order. "namespaces" must come first; the rest are
# applied after (enforced with an explicit CDK dependency below).
NAMESPACE_GROUP = "namespaces"
WORKLOAD_GROUPS = ["triton", "vllm", "worker", "scheduler", "api"]


def _load_group(group: str) -> list[dict]:
    """Load and flatten every YAML document under infra/k8s/<group>/."""
    group_dir = K8S_DIR / group
    docs: list[dict] = []
    for path in sorted(group_dir.glob("*.yaml")):
        with path.open() as f:
            for doc in yaml.safe_load_all(f):
                if doc:  # skip empty documents (e.g. trailing "---")
                    docs.append(doc)
    return docs


class WorkloadsStack(Stack):
    """Deploys the pipeline pods onto an existing EKS cluster."""

    def __init__(
        self,
        scope: Construct,
        construct_id: str,
        *,
        cluster: eks.ICluster,
        **kwargs,
    ) -> None:
        super().__init__(scope, construct_id, **kwargs)

        # Namespaces first.
        namespaces = cluster.add_manifest("Namespaces", *_load_group(NAMESPACE_GROUP))

        # Then each workload group, ordered after the namespaces exist.
        for group in WORKLOAD_GROUPS:
            docs = _load_group(group)
            if not docs:
                continue
            manifest = cluster.add_manifest(group.capitalize(), *docs)
            manifest.node.add_dependency(namespaces)
