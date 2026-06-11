"""EKS cluster stack: VPC, control plane, CPU + GPU managed nodegroups, GPU device plugin."""

from aws_cdk import (
    CfnOutput,
    Stack,
)
from aws_cdk import aws_ec2 as ec2
from aws_cdk import aws_eks as eks
from aws_cdk.lambda_layer_kubectl_v31 import KubectlV31Layer
from constructs import Construct

# Kubernetes version for the control plane. Keep in sync with the pinned
# KubectlV31Layer above (v31 -> 1.31) and requirements.txt.
KUBERNETES_VERSION = eks.KubernetesVersion.V1_31

# Instance types per nodegroup.
CPU_INSTANCE_TYPE = "m5.large"      # worker / scheduler / api / system pods
GPU_INSTANCE_TYPE = "g4dn.xlarge"   # triton / vllm (1x NVIDIA T4)

# Taint applied to GPU nodes so only GPU workloads (with a matching toleration)
# land on them.
GPU_TAINT_KEY = "nvidia.com/gpu"
GPU_TAINT_VALUE = "true"


class ClusterStack(Stack):
    """Provisions the EKS cluster and its node capacity.

    Exposes ``self.cluster`` so other stacks (e.g. WorkloadsStack) can attach
    Kubernetes manifests to the same cluster.
    """

    def __init__(self, scope: Construct, construct_id: str, **kwargs) -> None:
        super().__init__(scope, construct_id, **kwargs)

        # Dedicated VPC across 2 AZs (one NAT gateway to keep cost down).
        vpc = ec2.Vpc(self, "Vpc", max_azs=2, nat_gateways=1)

        # Control plane. Default capacity is disabled so we manage nodegroups
        # explicitly below. The deploying principal is granted cluster-admin via
        # EKS access entries (bootstrapClusterCreatorAdminPermissions, default).
        self.cluster = eks.Cluster(
            self,
            "Cluster",
            version=KUBERNETES_VERSION,
            kubectl_layer=KubectlV31Layer(self, "KubectlLayer"),
            vpc=vpc,
            default_capacity=0,
        )

        # CPU nodegroup for the Go services and cluster system workloads.
        self.cluster.add_nodegroup_capacity(
            "CpuNodegroup",
            instance_types=[ec2.InstanceType(CPU_INSTANCE_TYPE)],
            min_size=2,
            max_size=4,
            desired_size=2,
            labels={"workload": "cpu"},
        )

        # GPU nodegroup for inference servers. Uses the EKS GPU-optimized AMI
        # (NVIDIA drivers pre-installed) and is tainted so only GPU pods schedule.
        self.cluster.add_nodegroup_capacity(
            "GpuNodegroup",
            instance_types=[ec2.InstanceType(GPU_INSTANCE_TYPE)],
            ami_type=eks.NodegroupAmiType.AL2_X86_64_GPU,
            min_size=0,
            max_size=2,
            desired_size=1,
            labels={"workload": "gpu"},
            taints=[
                eks.TaintSpec(
                    effect=eks.TaintEffect.NO_SCHEDULE,
                    key=GPU_TAINT_KEY,
                    value=GPU_TAINT_VALUE,
                )
            ],
        )

        # NVIDIA device plugin: advertises nvidia.com/gpu to the scheduler.
        # The chart's DaemonSet tolerates the GPU taint by default.
        self.cluster.add_helm_chart(
            "NvidiaDevicePlugin",
            chart="nvidia-device-plugin",
            release="nvidia-device-plugin",
            repository="https://nvidia.github.io/k8s-device-plugin",
            namespace="kube-system",
            version="0.16.2",
        )

        CfnOutput(self, "ClusterName", value=self.cluster.cluster_name)
        CfnOutput(
            self,
            "UpdateKubeconfigCommand",
            value=(
                f"aws eks update-kubeconfig --name {self.cluster.cluster_name} "
                f"--region {self.region}"
            ),
        )
