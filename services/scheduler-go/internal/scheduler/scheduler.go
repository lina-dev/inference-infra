package scheduler

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Scheduler struct {
	sqs         *sqs.Client
	k8s         *kubernetes.Clientset
	log         *zap.Logger
	queueURL    string
	namespace   string
	deployment  string
	scaleUpAt   int32
	scaleDownAt int32
	minReplicas int32
	maxReplicas int32
}

func New(ctx context.Context, log *zap.Logger) (*Scheduler, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	k8sCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("k8s in-cluster config: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	return &Scheduler{
		sqs:         sqs.NewFromConfig(awsCfg),
		k8s:         k8sClient,
		log:         log,
		queueURL:    getEnv("SQS_QUEUE_URL", ""),
		namespace:   getEnv("WORKER_NAMESPACE", "inference"),
		deployment:  getEnv("WORKER_DEPLOYMENT", "worker"),
		scaleUpAt:   int32(envInt("SCALE_UP_THRESHOLD", 5)),
		scaleDownAt: int32(envInt("SCALE_DOWN_THRESHOLD", 0)),
		minReplicas: int32(envInt("MIN_REPLICAS", 1)),
		maxReplicas: int32(envInt("MAX_REPLICAS", 20)),
	}, nil
}

// Reconcile reads SQS depth and adjusts worker deployment replicas.
func (s *Scheduler) Reconcile(ctx context.Context) error {
	depth, err := s.queueDepth(ctx)
	if err != nil {
		return fmt.Errorf("queue depth: %w", err)
	}
	s.log.Info("queue depth", zap.Int64("depth", depth))

	deploy, err := s.k8s.AppsV1().Deployments(s.namespace).Get(ctx, s.deployment, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	current := *deploy.Spec.Replicas
	desired := s.calcReplicas(int32(depth), current)

	if desired == current {
		return nil
	}

	s.log.Info("scaling workers", zap.Int32("from", current), zap.Int32("to", desired))
	return s.scale(ctx, deploy, desired)
}

func (s *Scheduler) calcReplicas(depth, current int32) int32 {
	switch {
	case depth > s.scaleUpAt:
		desired := depth / 2
		if desired > s.maxReplicas {
			desired = s.maxReplicas
		}
		return desired
	case depth <= s.scaleDownAt:
		return s.minReplicas
	default:
		return current
	}
}

func (s *Scheduler) scale(ctx context.Context, deploy *appsv1.Deployment, replicas int32) error {
	deploy.Spec.Replicas = aws.Int32(replicas)
	_, err := s.k8s.AppsV1().Deployments(s.namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}

func (s *Scheduler) queueDepth(ctx context.Context) (int64, error) {
	resp, err := s.sqs.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       &s.queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{"ApproximateNumberOfMessages"},
	})
	if err != nil {
		return 0, err
	}
	v, ok := resp.Attributes["ApproximateNumberOfMessages"]
	if !ok {
		return 0, fmt.Errorf("attribute not returned")
	}
	return strconv.ParseInt(v, 10, 64)
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}
