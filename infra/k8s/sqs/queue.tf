resource "aws_sqs_queue" "audio_jobs" {
  name                       = "audio-jobs"
  visibility_timeout_seconds = 300
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.audio_jobs_dlq.arn
    maxReceiveCount     = 3
  })

  tags = {
    Service = "inference-infra"
  }
}

resource "aws_sqs_queue" "audio_jobs_dlq" {
  name                      = "audio-jobs-dlq"
  message_retention_seconds = 1209600 # 14 days
}

output "queue_url" {
  value = aws_sqs_queue.audio_jobs.url
}

output "queue_arn" {
  value = aws_sqs_queue.audio_jobs.arn
}
