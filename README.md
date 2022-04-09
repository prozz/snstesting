# snstesting

![test](https://github.com/prozz/snstesting/workflows/test/badge.svg?branch=main)
![golangci-lint](https://github.com/prozz/snstesting/workflows/lint/badge.svg?branch=main)

Package `snstesting` simplifies checking what messages arrive at any SNS topic from the inside of your integration tests.
It does it by subscribing to SNS via ad-hoc SQS queue that is cleaned-up after the test.

## Installation

```shell
go get github.com/prozz/snstesting
```

## Usage

```go
// make sure you can access AWS
cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
    t.Fatalf("configuration error: %v ", err)
}

// subscribe to SNS and cleanup at the end
receive, cleanup := snstesting.New(t, cfg, topicName)
defer cleanup()

// fire your process here, whatever it is ;)

// get single message from SNS and examine it, repeat if needed
msg := receive()
assert.NotEmpty(t, msg)
```

In case you need more control over error handling, context or long polling settings, please use `snstesting.NewSubscriber` directly.

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
Please make sure to update tests.

## License
[MIT](https://choosealicense.com/licenses/mit/)