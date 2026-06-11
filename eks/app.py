#!/usr/bin/env python3
"""CDK app entrypoint for the inference-infra EKS environment.

Synthesizes two stacks:
  * InferenceEksCluster   - VPC, EKS control plane, CPU + GPU nodegroups.
  * InferenceEksWorkloads - the pipeline pods (loaded from infra/k8s manifests).
"""

import os

import aws_cdk as cdk

from stacks.cluster_stack import ClusterStack
from stacks.workloads_stack import WorkloadsStack

app = cdk.App()

env = cdk.Environment(
    account=os.environ.get("CDK_DEFAULT_ACCOUNT"),
    region=os.environ.get("CDK_DEFAULT_REGION"),
)

cluster_stack = ClusterStack(app, "InferenceEksCluster", env=env)

workloads_stack = WorkloadsStack(
    app,
    "InferenceEksWorkloads",
    cluster=cluster_stack.cluster,
    env=env,
)
workloads_stack.add_dependency(cluster_stack)

app.synth()
