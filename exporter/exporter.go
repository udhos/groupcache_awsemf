// Package exporter implements exporter for groupcache.
package exporter

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/udhos/aws-emf/emf"
	"github.com/udhos/cloudwatchlog/cwlog"
	"github.com/udhos/groupcache_exporter"
)

// Options define exporter options.
type Options struct {
	// Application is required application name.
	Application string

	// AwsConfig is optional aws config for sending EMF metrics to CloudWatch logs.
	// If unspecified, EMF metrics are sent to stdout.
	// AwsConfig can be created with config.LoadDefaultConfig() from importing "github.com/aws/aws-sdk-go-v2/config".
	AwsConfig *aws.Config

	// RetentionInDays defaults to 30.
	RetentionInDays int32

	// ListGroups function must provide current list of groupcache groups.
	ListGroups func() []groupcache_exporter.GroupStatistics

	// ExportInterval defaults to 1 minute.
	ExportInterval time.Duration

	// HostnameTagKey defaults to "pod_name".
	HostnameTagKey string

	// EnableHostnameTag adds hostname tag $HostnameTagKey:$hostname.
	// Default is false because if you have many distincts hostnames over time,
	// adding them to metrics might generate high metric cardinality that
	// would increase CloudWatch costs.
	EnableHostnameTag bool

	// Debug enables debugging logs.
	Debug bool

	// Namespace defaults to groupcache.
	Namespace string

	// LogGroup defaults to /groupcache/{Application}.
	LogGroup string
}

// Exporter exports stats.
type Exporter struct {
	options       Options
	done          chan struct{}
	hostname      string
	metricContext *emf.Metric
	cwlogClient   *cwlog.Log
}

// New creates an exporter.
func New(options Options) (*Exporter, error) {

	if options.Application == "" {
		return nil, errors.New("option field Application is required")
	}

	if options.ExportInterval == 0 {
		options.ExportInterval = time.Minute
	}

	if options.Namespace == "" {
		options.Namespace = "groupcache"
	}

	if options.LogGroup == "" {
		options.LogGroup = "/groupcache/" + options.Application
	}

	var hostname string

	if options.EnableHostnameTag {
		if options.HostnameTagKey == "" {
			options.HostnameTagKey = "pod_name"
		}
		h, err := os.Hostname()
		if err != nil {
			slog.Error(fmt.Sprintf("groupcache_awsemf: exporter.New: hostname error: %v", err))
		}
		hostname = h
	}

	e := &Exporter{
		options:       options,
		done:          make(chan struct{}),
		hostname:      hostname,
		metricContext: emf.New(emf.Options{}),
	}

	if options.AwsConfig != nil {
		cw, errCwlog := cwlog.New(cwlog.Options{
			AwsConfig:       *options.AwsConfig,
			LogGroup:        options.LogGroup,
			LogStream:       options.Application,
			RetentionInDays: options.RetentionInDays,
		})
		if errCwlog != nil {
			return nil, errCwlog
		}
		e.cwlogClient = cw
	}

	go func() {
		for {
			select {
			case <-e.done:
				return
			default:
				e.exportOnce()
			}
			time.Sleep(options.ExportInterval)
		}
	}()

	return e, nil
}

// exportOnce all metrics once.
func (e *Exporter) exportOnce() {
	for _, g := range e.options.ListGroups() {
		e.exportGroup(g)
	}
	if e.cwlogClient == nil {
		// send metrics to stdout
		e.metricContext.Println()
	} else {
		// send metrics to cloudwatch logs
		events := e.metricContext.CloudWatchLogEvents()
		if err := e.cwlogClient.PutLogEvents(events); err != nil {
			slog.Error(fmt.Sprintf("groupcache_awsemf export error: %v", err))
		}
	}
}

// Close finishes the exporter.
func (e *Exporter) Close() error {
	close(e.done)
	return nil
}

func (e *Exporter) exportGroup(g groupcache_exporter.GroupStatistics) {
	groupName := g.Name()
	dimensions := map[string]string{
		"application": e.options.Application,
		"group":       groupName,
	}

	if e.hostname != "" {
		dimensions[e.options.HostnameTagKey] = e.hostname
	}

	stats := g.Collect() // grab current metrics

	if e.options.Debug {
		slog.Info("exportGroup",
			"group", groupName,
			"stats", stats,
		)
	}

	metricGets := emf.MetricDefinition{Name: "gets"}
	metricHits := emf.MetricDefinition{Name: "hits"}
	metricGetFromPeersLatencyLower := emf.MetricDefinition{Name: "get_from_peers_latency_slowest_milliseconds"}
	metricPeerLoads := emf.MetricDefinition{Name: "peer_loads"}
	metricPeerErrors := emf.MetricDefinition{Name: "peer_errors"}
	metricLoads := emf.MetricDefinition{Name: "loads"}
	metricLoadsDeduped := emf.MetricDefinition{Name: "loads_deduped"}
	metricLocalLoads := emf.MetricDefinition{Name: "local_load"}
	metricLocalLoadsErrs := emf.MetricDefinition{Name: "local_load_errs"}
	metricServerRequests := emf.MetricDefinition{Name: "server_requests"}
	metricCrosstalkRefusals := emf.MetricDefinition{Name: "crosstalk_refusals"}

	namespace := e.options.Namespace

	group := stats.Group

	e.metricContext.Record(namespace, metricGets, dimensions, int(group.CounterGets))
	e.metricContext.Record(namespace, metricHits, dimensions, int(group.CounterHits))
	e.metricContext.Record(namespace, metricGetFromPeersLatencyLower, dimensions, int(group.GaugeGetFromPeersLatencyLower))
	e.metricContext.Record(namespace, metricPeerLoads, dimensions, int(group.CounterPeerLoads))
	e.metricContext.Record(namespace, metricPeerErrors, dimensions, int(group.CounterPeerErrors))
	e.metricContext.Record(namespace, metricLoads, dimensions, int(group.CounterLoads))
	e.metricContext.Record(namespace, metricLoadsDeduped, dimensions, int(group.CounterLoadsDeduped))
	e.metricContext.Record(namespace, metricLocalLoads, dimensions, int(group.CounterLocalLoads))
	e.metricContext.Record(namespace, metricLocalLoadsErrs, dimensions, int(group.CounterLocalLoadsErrs))
	e.metricContext.Record(namespace, metricServerRequests, dimensions, int(group.CounterServerRequests))
	e.metricContext.Record(namespace, metricCrosstalkRefusals, dimensions, int(group.CounterCrosstalkRefusals))

	e.exportGroupType(stats.Main, namespace, dimensions, "main")
	e.exportGroupType(stats.Hot, namespace, dimensions, "hot")
}

func (e *Exporter) exportGroupType(stats groupcache_exporter.CacheTypeStats,
	namespace string,
	dimensions map[string]string,
	cacheType string) {

	dimensions["type"] = cacheType

	metricCacheItems := emf.MetricDefinition{Name: "cache_items"}
	metricCacheBytes := emf.MetricDefinition{Name: "cache_bytes"}
	metricCacheGets := emf.MetricDefinition{Name: "cache_gets"}
	metricCacheHits := emf.MetricDefinition{Name: "cache_hits"}
	metricCacheEvictions := emf.MetricDefinition{Name: "cache_evictions"}
	metricCacheEvictionsNonExpired := emf.MetricDefinition{Name: "cache_evictions_nonexpired"}

	e.metricContext.Record(namespace, metricCacheItems, dimensions, int(stats.GaugeCacheItems))
	e.metricContext.Record(namespace, metricCacheBytes, dimensions, int(stats.GaugeCacheBytes))
	e.metricContext.Record(namespace, metricCacheGets, dimensions, int(stats.CounterCacheGets))
	e.metricContext.Record(namespace, metricCacheHits, dimensions, int(stats.CounterCacheHits))
	e.metricContext.Record(namespace, metricCacheEvictions, dimensions, int(stats.CounterCacheEvictions))
	e.metricContext.Record(namespace, metricCacheEvictionsNonExpired, dimensions, int(stats.CounterCacheEvictionsNonExpired))
}
