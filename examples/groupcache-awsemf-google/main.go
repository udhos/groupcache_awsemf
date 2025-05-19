// Package main implements the example.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"log/slog"

	"github.com/golang/groupcache"
	"github.com/udhos/groupcache_awsemf/exporter"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/google"
)

func main() {

	cache := startGroupcache()

	//
	// metrics exporter
	//

	exporter, errExport := exporter.New(exporter.Options{
		Application: path.Base(os.Args[0]),
		ListGroups: func() []groupcache_exporter.GroupStatistics {
			return google.ListGroups([]*groupcache.Group{cache})
		},
		ExportInterval: 20 * time.Second,
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
		query(cache, "/etc/passwd")             // repeat key
		query(cache, fmt.Sprintf("fake-%d", i)) // always miss, and gets evicted
		time.Sleep(interval)
	}
}

func query(cache *groupcache.Group, key string) {
	begin := time.Now()
	var dst []byte
	cache.Get(context.TODO(), key, groupcache.AllocatingByteSliceSink(&dst))
	elap := time.Since(begin)

	slog.Info(fmt.Sprintf("cache answer: bytes=%d elapsed=%v",
		len(dst), elap))
}
