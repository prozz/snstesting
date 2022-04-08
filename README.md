# snstesting

Package snstesting simplifies checking what messages arrive at any SNS topic from the inside of your integration tests.

# Usage

```go
    // make sure you can access AWS
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        t.Fatalf("configuration error: %v ", err)
    }

    // subscribe to SNS and cleanup at the end
    subscriber, cleanupFn := snstesting.New(t, cfg, topicName)
    defer cleanupFn()

    // fire your process here, whatever it is ;)

    // get single message from SNS and examine it, repeat if needed
    msg, err := subscriber.Receive(ctx)
    assert.NoError(t, err)
    assert.NotEmpty(t, msg)
```