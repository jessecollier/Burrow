package main

import (
	log "github.com/cihub/seelog"
	"github.com/DataDog/datadog-go/statsd"
	"os"
	"time"
	"fmt"
)

type MetricsReporter struct {
	app            *ApplicationContext
	statsd         *statsd.Client
	emitInterval   int64
	emitTicker     *time.Ticker
	quitChan       chan struct{}
}

func NewMetricsReporter(app *ApplicationContext) error {
	
	m := &MetricsReporter{
		app:            app,
		emitInterval:   app.Config.Metrics.EmitInterval,
		quitChan:       make(chan struct{}),
	}


	if app.Config.Metrics.StatsdHost != "" {
		statsdhost := app.Config.Metrics.StatsdHost
		statsdport := "8125"

		if app.Config.Metrics.StatsdPort != "" {
			statsdport = app.Config.Metrics.StatsdPort
		}

		var err error 
		m.statsd, err = statsd.New(fmt.Sprintf("%s:%s", statsdhost, statsdport))
		if err != nil {
			log.Critical(err)
		}
	}
	app.MetricsReporter = m
	return nil
}


func StartMetrics(app *ApplicationContext) {
	m := app.MetricsReporter
	// Do not proceed until we get the Zookeeper lock
	err := app.MetricsLock.Lock()
	if err != nil {
		log.Criticalf("Cannot get ZK m lock: %v", err)
		os.Exit(1)
	}
	log.Info("Acquired Zookeeper metrics lock")

	// Set a ticker to refresh the group list periodically
	m.emitTicker = time.NewTicker(time.Duration(m.emitInterval) * time.Second)

	// Main loop to handle refreshes and evaluation responses
OUTERLOOP:
	for {
		select {
		case <-m.quitChan:
			break OUTERLOOP
		case <-m.emitTicker.C:
			m.emitTopicCount()
		}
	}
}

func StopMetrics(app *ApplicationContext) {
	// Ignore errors on unlock - we're quitting anyways, and it might not be locked
	app.MetricsLock.Unlock()
	m := app.MetricsReporter
	if m.emitTicker != nil {
		m.emitTicker.Stop()
	}
	close(m.quitChan)
	// TODO stop all ms
}

func (mr *MetricsReporter) emitTopicCount() {
	for _,cluster := range mr.app.Clusters {
		topics,_ := cluster.Client.client.Topics()
		for _, topic := range topics {
			tags := []string{ fmt.Sprintf("topic:%s",topic) }
			_ = mr.statsd.Gauge("kafka.burrow.topic.count", float64(1), tags, 1)
		}
	}
}
