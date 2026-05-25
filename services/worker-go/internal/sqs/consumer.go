package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"
)

// Job is the message payload read from SQS.
type Job struct {
	JobID string `json:"job_id"`
	S3Key string `json:"s3_key"`
}

// Handler is called for each dequeued job. A non-nil error leaves the message
// in-flight — it will be retried after the visibility timeout expires.
type Handler func(ctx context.Context, job Job) error

// Consumer long-polls a single SQS queue and dispatches jobs to a Handler.
type Consumer struct {
	client   *awssqs.Client
	queueURL string
	handler  Handler
	log      *zap.Logger
}

func NewConsumer(client *awssqs.Client, queueURL string, handler Handler, log *zap.Logger) *Consumer {
	return &Consumer{client: client, queueURL: queueURL, handler: handler, log: log}
}

// Run blocks until ctx is cancelled, processing messages one at a time.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgs, err := c.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:            &c.queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			c.log.Error("receive message", zap.Error(err))
			continue
		}

		for _, msg := range msgs.Messages {
			var job Job
			if err := json.Unmarshal([]byte(*msg.Body), &job); err != nil {
				c.log.Error("unmarshal message", zap.Error(err), zap.String("body", *msg.Body))
				continue
			}

			if err := c.handler(ctx, job); err != nil {
				c.log.Error("job failed", zap.Error(err), zap.String("job_id", job.JobID))
				continue
			}

			if _, err := c.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
				QueueUrl:      &c.queueURL,
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				c.log.Warn("delete message failed", zap.Error(err), zap.String("job_id", job.JobID))
			}
		}
	}
}

// QueueDepth returns the approximate number of visible messages.
func QueueDepth(ctx context.Context, client *awssqs.Client, queueURL string) (int64, error) {
	const attrName = "ApproximateNumberOfMessages"
	resp, err := client.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{
		QueueUrl:       &queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{attrName},
	})
	if err != nil {
		return 0, err
	}
	v, ok := resp.Attributes[attrName]
	if !ok {
		return 0, fmt.Errorf("attribute %s not returned", attrName)
	}
	var depth int64
	fmt.Sscanf(v, "%d", &depth)
	return depth, nil
}
