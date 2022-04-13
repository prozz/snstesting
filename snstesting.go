// Package snstesting simplifies checking what messages arrive at any SNS topic from the inside of your integration tests.
package snstesting

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Subscriber for checking what arrives at any SNS topic in an integration testing setting.
// It does it through temporary SQS queue and ad-hoc subscription that may be easily cleaned up after the test.
// Temporary queues are prefixed with 'snstesting'.
type Subscriber struct {
	SNS    SNSAPI
	SQS    SQSAPI
	Config Config
}

// Config describes both temporarily generated and existing resources used by the ad-hoc SNS checking mechanism.
type Config struct {
	TopicName       string
	TopicARN        string
	QueueName       string
	QueueURL        string
	QueueARN        string
	SubscriptionARN string
}

// ReceiveFn checks for message that arrived at SNS (via ad-hoc SQS queue), can be called repeatedly.
type ReceiveFn func() string

// New creates Subscriber for testing purposes based on provided AWS configuration.
// In case of an error, t.Fatal is executed.
// In case more control is needed over Subscriber, or it's Config, please use NewSubscriber.
func New(t *testing.T, cfg aws.Config, topicName string) ReceiveFn {
	t.Helper()

	ctx := context.Background()

	SNS := sns.NewFromConfig(cfg)
	SQS := sqs.NewFromConfig(cfg)

	s, err := NewSubscriber(ctx, SNS, SQS, topicName)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		err := s.Cleanup(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})

	return func() string {
		msg, err := s.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return msg
	}
}

// NewSubscriber creates Subscriber instance for ad-hoc subscribing to SNS topic.
func NewSubscriber(ctx context.Context, SNS SNSAPI, SQS SQSAPI, topicName string) (Subscriber, error) {
	topicArn, err := findTopicArn(ctx, SNS, topicName)
	if err != nil {
		return Subscriber{}, err
	}

	testingQueueName := fmt.Sprintf("snstesting_%s", rndString(20))
	createQueueOutput, err := SQS.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(testingQueueName),
	})
	if err != nil {
		return Subscriber{}, err
	}

	queueAttrsOutput, err := SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       createQueueOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameQueueArn},
	})
	if err != nil {
		return Subscriber{}, fmt.Errorf("get queue attributes failure: %v",
			combineErr(err, cleanupQueue(ctx, SQS, *createQueueOutput.QueueUrl)))
	}

	queueArn := queueAttrsOutput.Attributes[string(types.QueueAttributeNameQueueArn)]

	policy := policyTemplate
	policy = strings.ReplaceAll(policy, "<<TOPIC_ARN>>", topicArn)
	policy = strings.ReplaceAll(policy, "<<QUEUE_ARN>>", queueArn)

	_, err = SQS.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl: createQueueOutput.QueueUrl,
		Attributes: map[string]string{
			string(types.QueueAttributeNamePolicy): policy,
		},
	})
	if err != nil {
		return Subscriber{}, fmt.Errorf("set queue attributes failure: %v",
			combineErr(err, cleanupQueue(ctx, SQS, *createQueueOutput.QueueUrl)))
	}

	subscribeOutput, err := SNS.Subscribe(ctx, &sns.SubscribeInput{
		Protocol: aws.String("sqs"),
		TopicArn: aws.String(topicArn),
		Endpoint: aws.String(queueArn),
	})
	if err != nil {
		return Subscriber{}, fmt.Errorf("subscribe failure: %v",
			combineErr(err, cleanupQueue(ctx, SQS, *createQueueOutput.QueueUrl)))
	}

	return Subscriber{
		SNS: SNS,
		SQS: SQS,
		Config: Config{
			TopicName:       topicName,
			TopicARN:        topicArn,
			QueueName:       testingQueueName,
			QueueURL:        *createQueueOutput.QueueUrl,
			QueueARN:        queueArn,
			SubscriptionARN: *subscribeOutput.SubscriptionArn,
		},
	}, nil
}

// Receive receives single message that was published on SNS.
func (s Subscriber) Receive(ctx context.Context) (string, error) {
	receiveOut, err := s.SQS.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.Config.QueueURL),
		MaxNumberOfMessages: 1,
		VisibilityTimeout:   3600, // just hide msg for long enough, could be moved to Config for easy manipulation
		WaitTimeSeconds:     3,
	})
	if err != nil {
		return "", err
	}
	if len(receiveOut.Messages) > 0 {
		return *receiveOut.Messages[0].Body, nil
	}
	return "", nil
}

// Cleanup unsubscribes temporary SQS queue from SNS and removes it.
func (s Subscriber) Cleanup(ctx context.Context) error {
	return combineErr(unsubscribe(ctx, s.SNS, s.Config.SubscriptionARN), cleanupQueue(ctx, s.SQS, s.Config.QueueURL))
}

func cleanupQueue(ctx context.Context, SQS SQSAPI, queueURL string) error {
	_, err := SQS.DeleteQueue(ctx, &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	})
	return err
}

func unsubscribe(ctx context.Context, SNS SNSAPI, subscriptionARN string) error {
	_, err := SNS.Unsubscribe(ctx, &sns.UnsubscribeInput{
		SubscriptionArn: aws.String(subscriptionARN),
	})
	return err
}

// iterating over all sns topics may be a waste of time
// according to stackoverflow its ok create topic with same name again to get ARN
// TODO explore if this is best practice, risk: some sns props may get overwritten by accident?
func findTopicArn(ctx context.Context, cli SNSAPI, topicName string) (string, error) {
	var nextToken *string
	for {
		out, err := cli.ListTopics(ctx, &sns.ListTopicsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return "", err
		}

		for _, topic := range out.Topics {
			if topic.TopicArn != nil && strings.Contains(*topic.TopicArn, topicName) {
				return *topic.TopicArn, nil
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			return "", fmt.Errorf("topic %s not found", topicName)
		}
	}
}

// simple random string generation to avoid external deps
func rndString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// simple error combine to avoid external deps
func combineErr(errs ...error) error {
	var v []string
	for _, err := range errs {
		if err != nil {
			v = append(v, err.Error())
		}
	}
	if len(v) > 0 {
		return fmt.Errorf(strings.Join(v, ","))
	}
	return nil
}

var policyTemplate = `
{
   "Version": "2012-10-17",
   "Statement": [
     {
       "Sid": "allow-sns-messages",
       "Effect": "Allow",
       "Principal": "*",
       "Resource": "<<QUEUE_ARN>>",
       "Action": "SQS:SendMessage",
       "Condition": {
         "ArnEquals": {
           "aws:SourceArn": "<<TOPIC_ARN>>"
         }
       }
     }
  ]
}
`
