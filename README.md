[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/udhos/groupcache_awsemf/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/udhos/groupcache_awsemf)](https://goreportcard.com/report/github.com/udhos/groupcache_awsemf)
[![Go Reference](https://pkg.go.dev/badge/github.com/udhos/groupcache_awsemf.svg)](https://pkg.go.dev/github.com/udhos/groupcache_awsemf)

# groupcache_awsemf

[groupcache_awsemf](https://github.com/udhos/groupcache_awsemf) exports [groupcache](https://github.com/modernprogram/groupcache) metrics to AWS CloudWatch Logs using [Embedded Metric Format](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format.html).

# Synopsis

```go
import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/modernprogram/groupcache/v2"
	"github.com/udhos/groupcache_awsemf/exporter"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/modernprogram"
)

workspace := groupcache.NewWorkspace()

caches := startGroupcache(workspace)

//
// metrics exporter
//

var awsConfig *aws.Config
if cloudwatchSend {
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        log.Fatalf("aws sdk config error: %v", err)
    }
    awsConfig = &cfg
}

exporter, errExport := exporter.New(exporter.Options{
    Application: "my-application-name",
    AwsConfig:   awsConfig, // if nil, metric is echoed to stdout
    ListGroups: func() []groupcache_exporter.GroupStatistics {
        return modernprogram.ListGroups(workspace)
    },
    ExportInterval: 20 * time.Second,
})
if errExport != nil {
    log.Fatal(errExport)
}
defer exporter.Close()
```

# Examples

- [./examples/groupcache-awsemf-google/main.go](./examples/groupcache-awsemf-google/main.go)
- [./examples/groupcache-awsemf-mailgun/main.go](./examples/groupcache-awsemf-mailgun/main.go)
- [./examples/groupcache-awsemf-modernprogram/main.go](./examples/groupcache-awsemf-modernprogram/main.go)

# Findind EMF metrics sent to CloudWatch

By default:

1 - EMF logs are issued to log group `/groupache/{Application}`. See option `LogGroup` to change the log group.

2 - EMF metrics are issued to namespace `groupcache`. See option `Namespace` to change the namespace.

3 - EMF metrics have dimension `application={Application}`. See option `Application` to set the application name.
