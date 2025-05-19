// Package main implements the example.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/modernprogram/groupcache/v2"
	"github.com/udhos/groupcache_awsemf/exporter"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/modernprogram"
)

func main() {

	var debugExporter bool
	var cloudwatchSend bool
	flag.BoolVar(&debugExporter, "debugExporter", false, "enable exporter debug")
	flag.BoolVar(&cloudwatchSend, "send", false, "send to cloudwatch")
	flag.Parse()

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
		Application: path.Base(os.Args[0]),
		AwsConfig:   awsConfig,
		ListGroups: func() []groupcache_exporter.GroupStatistics {
			return modernprogram.ListGroups(workspace)
		},
		ExportInterval: 20 * time.Second,
		Debug:          debugExporter,
	})
	if errExport != nil {
		log.Fatal(errExport)
	}
	defer exporter.Close()

	//
	// query cache periodically
	//

	const interval = 5 * time.Second

	for i := 0; ; i++ {
		for _, cache := range caches {
			query(cache, "/etc/passwd")             // repeat key
			query(cache, fmt.Sprintf("fake-%d", i)) // always miss, and gets evicted
			time.Sleep(interval)
		}
	}
}

func query(cache *groupcache.Group, key string) {
	begin := time.Now()
	var dst []byte
	cache.Get(context.TODO(), key, groupcache.AllocatingByteSliceSink(&dst), nil)
	elap := time.Since(begin)

	slog.Info(fmt.Sprintf("cache answer: bytes=%d elapsed=%v",
		len(dst), elap))
}
