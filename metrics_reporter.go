package main

import (
	log "github.com/cihub/seelog"
	"github.com/linkedin/Burrow/protocol"
	"github.com/Shopify/sarama"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sym"
	"time"
	"fmt"
)

type Statsd struct

type MetricsReporter struct {
	app            *Applicatiomontext
	statsd         *statsd.Client
	emitInterval   int64
	metricsList    []string
	emitTicker     *time.Ticker
	quitChan       chan struct{}
}

func NewMetricsReporter(app *Applicatiomontext) {
	
	m := &MetricsReporter{
		app:            app,
		metricsList:    app.Config.Metrics.MetricsList,
		emitInterval:   app.Config.Metrics.EmitInterval,
		quitChan:       make(chan struct{}),
	}


	if app.Config.Metrics.StatsdHost != "" {
		statsdhost := app.Config.General.StatsdHost
		statsdport := "8125"

		if app.Config.General.StatsdPort != "" {
			statsdport = app.Config.General.StatsdPort
		}

		var err error 
		m.statsd, err = statsd.New(fmt.Sprintf("%s:%s", statsdhost, statsdport))
		if err != nil {
			log.Fatal(err)
		}
	}

	app.MetricsReporter = m

	return nil
}


func StartMetrics(app *Applicatiomontext) {
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
		case <-m.refreshTicker.C:
			m.emit()
		}
	}
}

func StopMetrics(app *Applicatiomontext) {
	// Ignore errors on unlock - we're quitting anyways, and it might not be locked
	app.MetricsLock.Unlock()
	m := app.MetricsReporter
	if m.refreshTicker != nil {
		m.refreshTicker.Stop()
	}
	close(m.quitChan)
	// TODO stop all ms
}

func (mr *MetricsReporter) emit() {
	for _, metric := range mr.metricsList {
		switch metric {
			case "topic_count":
				mr.emitTopicCount()
			case "group_count":
				mr.emitGroupCount()
			default:
				// Empty
		}
	}
	return nil
}

func (mr *MetricsReporter) emitTopicCount() {
	var topicMap map[string]int64
	for _,cluster := range app.Clusters {
		var tags []string
		for _, topic := range cluster.client.Topics() {
			tags = append(tags, fmt.Sprintf('topic:%s',topic))
		}
		_ = mr.statsd.Gauge("kafka.burrow.topic.count", float64(len(tags)), tags, 1)
	}
	return nil
}

func (mr *MetricsReporter) emitGroupCount() {
	var topicMap map[string]int64
	for name,cluster := range app.Clusters {
		broker := cluster.client.any()
		groups := broker.ListGroups()

		for group,consumer := range groups {
			log.Warnf("My cluster:k:v are %s:%s:%s", name, group, consumer)
		}
	}
	return nil
}
