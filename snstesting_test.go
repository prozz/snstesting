package snstesting_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/golang/mock/gomock"
	"github.com/prozz/snstesting"
	"github.com/prozz/snstesting/mock"
	"github.com/stretchr/testify/assert"
)

func TestNewSubscriber(t *testing.T) {
	ctx := context.Background()

	t.Run("error listing topics", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(nil, assert.AnError)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.Empty(t, subscriber)
	})

	t.Run("no topic found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				NextToken: aws.String("page 2"),
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar")},
				},
			}, nil)
		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: aws.String("page 2")}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:baz")},
				},
			}, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.Empty(t, subscriber)
	})

	t.Run("error creating queue", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(nil, assert.AnError)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.Empty(t, subscriber)
	})

	t.Run("error getting queue attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "get queue attributes failure: foo")
		assert.Empty(t, subscriber)
	})

	t.Run("error getting queue attributes and on cleanup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, errors.New("bar"))

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "get queue attributes failure: foo,bar")
		assert.Empty(t, subscriber)
	})

	t.Run("error setting queue attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "set queue attributes failure: foo")
		assert.Empty(t, subscriber)
	})

	t.Run("error setting queue attributes and on cleanup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, errors.New("bar"))

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "set queue attributes failure: foo,bar")
		assert.Empty(t, subscriber)
	})

	t.Run("error subscribing to sns", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(&sqs.SetQueueAttributesOutput{}, nil)

		SNS.EXPECT().Subscribe(ctx, &sns.SubscribeInput{
			Protocol: aws.String("sqs"),
			TopicArn: aws.String("arn:foo:bar:sometopic"),
			Endpoint: aws.String("arn:foo:bar:testingqueue"),
		}).Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "subscribe failure: foo")
		assert.Empty(t, subscriber)
	})

	t.Run("error subscribing to sns and on cleanup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(&sqs.SetQueueAttributesOutput{}, nil)

		SNS.EXPECT().Subscribe(ctx, &sns.SubscribeInput{
			Protocol: aws.String("sqs"),
			TopicArn: aws.String("arn:foo:bar:sometopic"),
			Endpoint: aws.String("arn:foo:bar:testingqueue"),
		}).Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, errors.New("bar"))

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.Error(t, err)
		assert.EqualError(t, err, "subscribe failure: foo,bar")
		assert.Empty(t, subscriber)
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(&sqs.SetQueueAttributesOutput{}, nil)

		SNS.EXPECT().Subscribe(ctx, &sns.SubscribeInput{
			Protocol: aws.String("sqs"),
			TopicArn: aws.String("arn:foo:bar:sometopic"),
			Endpoint: aws.String("arn:foo:bar:testingqueue"),
		}).Return(&sns.SubscribeOutput{
			SubscriptionArn: aws.String("arn:foo:bar:subscription"),
		}, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.NoError(t, err)
		assert.Equal(t, "sometopic", subscriber.Config.TopicName)
		assert.Equal(t, "arn:foo:bar:sometopic", subscriber.Config.TopicARN)
		assert.True(t, strings.HasPrefix(subscriber.Config.QueueName, "snstesting_"))
		assert.Len(t, subscriber.Config.QueueName, 31)
		assert.Equal(t, "http://queue.url", subscriber.Config.QueueURL)
		assert.Equal(t, "arn:foo:bar:testingqueue", subscriber.Config.QueueARN)
		assert.Equal(t, "arn:foo:bar:subscription", subscriber.Config.SubscriptionARN)
	})

	t.Run("success, paging of topics list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: nil}).
			Return(&sns.ListTopicsOutput{
				NextToken: aws.String("page 2"),
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar")},
				},
			}, nil)
		SNS.EXPECT().ListTopics(ctx, &sns.ListTopicsInput{NextToken: aws.String("page 2")}).
			Return(&sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:foo:bar:sometopic")},
				},
			}, nil)

		SQS.EXPECT().CreateQueue(ctx, gomock.AssignableToTypeOf(&sqs.CreateQueueInput{})).
			Do(func(ctx context.Context, input *sqs.CreateQueueInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueName) {
					assert.True(t, strings.HasPrefix(*input.QueueName, "snstesting_"))
				}
			}).
			Return(&sqs.CreateQueueOutput{
				QueueUrl: aws.String("http://queue.url"),
			}, nil)

		SQS.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String("http://queue.url"),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		}).Return(&sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:foo:bar:testingqueue",
			},
		}, nil)

		SQS.EXPECT().SetQueueAttributes(ctx, gomock.AssignableToTypeOf(&sqs.SetQueueAttributesInput{})).
			Do(func(ctx context.Context, input *sqs.SetQueueAttributesInput, opts ...*sns.Options) {
				if assert.NotNil(t, input.QueueUrl) {
					assert.Equal(t, "http://queue.url", *input.QueueUrl)
				}
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"Resource": "arn:foo:bar:testingqueue"`))
				assert.True(t, strings.Contains(input.Attributes["Policy"], `"aws:SourceArn": "arn:foo:bar:sometopic"`))
			}).
			Return(&sqs.SetQueueAttributesOutput{}, nil)

		SNS.EXPECT().Subscribe(ctx, &sns.SubscribeInput{
			Protocol: aws.String("sqs"),
			TopicArn: aws.String("arn:foo:bar:sometopic"),
			Endpoint: aws.String("arn:foo:bar:testingqueue"),
		}).Return(&sns.SubscribeOutput{
			SubscriptionArn: aws.String("arn:foo:bar:subscription"),
		}, nil)

		subscriber, err := snstesting.NewSubscriber(ctx, SNS, SQS, "sometopic")
		assert.NoError(t, err)
		assert.Equal(t, "sometopic", subscriber.Config.TopicName)
		assert.Equal(t, "arn:foo:bar:sometopic", subscriber.Config.TopicARN)
		assert.True(t, strings.HasPrefix(subscriber.Config.QueueName, "snstesting_"))
		assert.Equal(t, "http://queue.url", subscriber.Config.QueueURL)
		assert.Equal(t, "arn:foo:bar:testingqueue", subscriber.Config.QueueARN)
		assert.Equal(t, "arn:foo:bar:subscription", subscriber.Config.SubscriptionARN)
	})
}

func TestSubscriber_Receive(t *testing.T) {
	ctx := context.Background()

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SQS.EXPECT().ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String("http://queue.url"),
			MaxNumberOfMessages: 1,
			VisibilityTimeout:   3600,
			WaitTimeSeconds:     3,
		}).Return(&sqs.ReceiveMessageOutput{}, assert.AnError)

		subscriber := snstesting.Subscriber{
			SNS: SNS,
			SQS: SQS,
			Config: snstesting.Config{
				QueueURL: "http://queue.url",
			},
		}

		msg, err := subscriber.Receive(ctx)
		assert.Error(t, err)
		assert.Empty(t, msg)
	})

	t.Run("empty message", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SQS.EXPECT().ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String("http://queue.url"),
			MaxNumberOfMessages: 1,
			VisibilityTimeout:   3600,
			WaitTimeSeconds:     3,
		}).Return(&sqs.ReceiveMessageOutput{}, nil)

		subscriber := snstesting.Subscriber{
			SNS: SNS,
			SQS: SQS,
			Config: snstesting.Config{
				QueueURL: "http://queue.url",
			},
		}

		msg, err := subscriber.Receive(ctx)
		assert.NoError(t, err)
		assert.Empty(t, msg)
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SQS.EXPECT().ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String("http://queue.url"),
			MaxNumberOfMessages: 1,
			VisibilityTimeout:   3600,
			WaitTimeSeconds:     3,
		}).Return(&sqs.ReceiveMessageOutput{
			Messages: []sqstypes.Message{
				{Body: aws.String("this is the message!")},
			},
		}, nil)

		subscriber := snstesting.Subscriber{
			SNS: SNS,
			SQS: SQS,
			Config: snstesting.Config{
				QueueURL: "http://queue.url",
			},
		}

		msg, err := subscriber.Receive(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "this is the message!", msg)
	})
}

func TestSubscriber_Cleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().Unsubscribe(ctx, &sns.UnsubscribeInput{
			SubscriptionArn: aws.String("arn:foo:bar:subscription"),
		}).Return(nil, errors.New("foo"))

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(nil, errors.New("bar"))

		subscriber := snstesting.Subscriber{
			SNS: SNS,
			SQS: SQS,
			Config: snstesting.Config{
				QueueURL:        "http://queue.url",
				SubscriptionARN: "arn:foo:bar:subscription",
			},
		}

		err := subscriber.Cleanup(ctx)
		assert.EqualError(t, err, "foo,bar")
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		SQS := mock.NewMockSQSAPI(ctrl)
		SNS := mock.NewMockSNSAPI(ctrl)

		SNS.EXPECT().Unsubscribe(ctx, &sns.UnsubscribeInput{
			SubscriptionArn: aws.String("arn:foo:bar:subscription"),
		}).Return(&sns.UnsubscribeOutput{}, nil)

		SQS.EXPECT().DeleteQueue(ctx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://queue.url"),
		}).Return(&sqs.DeleteQueueOutput{}, nil)

		subscriber := snstesting.Subscriber{
			SNS: SNS,
			SQS: SQS,
			Config: snstesting.Config{
				QueueURL:        "http://queue.url",
				SubscriptionARN: "arn:foo:bar:subscription",
			},
		}

		err := subscriber.Cleanup(ctx)
		assert.NoError(t, err)
	})
}
