package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "SampleMetric"
)

type myCollector struct{}

var (
	exampleCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "example_count",
		Help:      "example counter help",
	})
	exampleGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "example_gauge",
		Help:      "example gauge help",
	})
)

func (c myCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- exampleCount.Desc()
	ch <- exampleGauge.Desc()
}

func (c myCollector) Collect(ch chan<- prometheus.Metric) {
	exampleValue := float64(12345)

	ch <- prometheus.MustNewConstMetric(
		exampleCount.Desc(),     //ここと
		prometheus.CounterValue, //ここは固定
		float64(exampleValue),   //ここが、グラフに表示させたい数値
	)
	ch <- prometheus.MustNewConstMetric(
		exampleGauge.Desc(),
		prometheus.GaugeValue,
		float64(exampleValue),
	)
}

var addr = flag.String("listen-address", "127.0.0.1:5000", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	var c myCollector
	prometheus.MustRegister(c)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
