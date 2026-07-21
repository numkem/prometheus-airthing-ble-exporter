package main

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	Timestamp   prometheus.Counter
	Humidity    prometheus.Gauge
	Temperature prometheus.Gauge
	RadonSTA    prometheus.Gauge
	RadonLTA    prometheus.Gauge
}

var (
	prometheusNamespace = "airthingsble"
)

type Exporter struct {
	Metrics *Metrics
}

func NewExporter(waveSerialNumber uint64, reg *prometheus.Registry) (*Exporter, error) {
	e := &Exporter{}

	e.Metrics = &Metrics{
		Timestamp: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: prometheusNamespace,
			Name:      "timestamp",
			Help:      "Timestamp of when the data was collected by the Wave",
			ConstLabels: prometheus.Labels{
				"serialNumber": fmt.Sprintf("%d", waveSerialNumber),
			},
		}),
		Humidity: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Name:      "humidity",
			Help:      "Humidity",
			Unit:      "%rH",
			ConstLabels: prometheus.Labels{
				"serialNumber": fmt.Sprintf("%d", waveSerialNumber),
			},
		}),
		Temperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Name:      "temperature",
			Help:      "Ambient Temperature",
			Unit:      "C",
			ConstLabels: prometheus.Labels{
				"serialNumber": fmt.Sprintf("%d", waveSerialNumber),
			},
		}),
		RadonSTA: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Name:      "radonsta",
			Help:      "Short term Radon average measured",
			Unit:      "Bq/m3",
			ConstLabels: prometheus.Labels{
				"serialNumber": fmt.Sprintf("%d", waveSerialNumber),
			},
		}),
		RadonLTA: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Name:      "radonlta",
			Help:      "Long term Radon average measured",
			Unit:      "Bq/m3",
			ConstLabels: prometheus.Labels{
				"serialNumber": fmt.Sprintf("%d", waveSerialNumber),
			},
		}),
	}

	reg.MustRegister(e.Metrics.Timestamp)
	reg.MustRegister(e.Metrics.Humidity)
	reg.MustRegister(e.Metrics.Temperature)
	reg.MustRegister(e.Metrics.RadonSTA)
	reg.MustRegister(e.Metrics.RadonLTA)

	return e, nil
}

func (e *Exporter) Collect() {
	log.Debugf("Collect :: currentReadValues: %+v\n", currentReadValues)

	e.Metrics.Timestamp.Add(float64(currentReadValues.Timestamp.Unix()))

	e.Metrics.Humidity.Set(currentReadValues.Humidity)
	e.Metrics.Temperature.Set(currentReadValues.Temperature)
	e.Metrics.RadonSTA.Set(float64(currentReadValues.RadonSTA))
	e.Metrics.RadonLTA.Set(float64(currentReadValues.RadonLTA))
}
