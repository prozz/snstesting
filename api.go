package snstesting

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSAPI shows part of SQS API needed to fulfill the contract.
type SQSAPI interface {
	CreateQueue(context.Context, *sqs.CreateQueueInput, ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error)
	DeleteQueue(context.Context, *sqs.DeleteQueueInput, ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error)
	GetQueueAttributes(context.Context, *sqs.GetQueueAttributesInput, ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) //nolint
	SetQueueAttributes(context.Context, *sqs.SetQueueAttributesInput, ...func(*sqs.Options)) (*sqs.SetQueueAttributesOutput, error) //nolint
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

// SNSAPI shows part of SNS API needed to fulfill the contract.
type SNSAPI interface {
	ListTopics(context.Context, *sns.ListTopicsInput, ...func(*sns.Options)) (*sns.ListTopicsOutput, error)
	Subscribe(context.Context, *sns.SubscribeInput, ...func(*sns.Options)) (*sns.SubscribeOutput, error)
	Unsubscribe(context.Context, *sns.UnsubscribeInput, ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
}
