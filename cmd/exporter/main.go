package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

func init() {
	prometheus.MustRegister(version.NewCollector("prometheus_airthings_ble_exporter"))
}

func main() {
	waveSerialNumber := flag.Uint64("serial", 0, "Serial number of the Wave 1 device")
	collectionDuration := flag.Duration("collection", time.Minute*30, "How often to read data from Wave")
	listenAddress := flag.String("address", ":9456", "Address to listen on for web interface and telemetry")
	metricsPath := flag.String("telemetry-path", "/metrics", "Path to expose metrics of the exporter")
	retries := flag.Int("retries", 3, "Number of retries if the poll failed")
	flag.Parse()

	if os.Getenv("DEBUG") != "" {
		log.SetLevel(log.DebugLevel)
		log.Debug("Set logging to debug level")
	}

	// Parameters validations
	if *retries < 0 {
		log.Error("Number of retries cannot be negative")
		os.Exit(1)
	}
	if *waveSerialNumber == uint64(0) {
		log.Fatal("Invalid serial number")
		os.Exit(1)
	}

	var adapter = bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		log.Fatalf("failed to enable BLE adapter: %v", err)
	}

	wave := NewWave(adapter, uint32(*waveSerialNumber))
	log.Info("Connecting to Wave...")
	if err := wave.Connect(3); err != nil {
		log.Fatalf("failed to connect to Wave device with serial %d: %v", *waveSerialNumber, err)
	}
	log.Info("Connected to Wave")

	tickCh := time.Tick(*collectionDuration)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	reg := prometheus.NewRegistry()
	exp, _ := NewExporter(*waveSerialNumber, reg)

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, *metricsPath, http.StatusMovedPermanently)
	})

	go func() {
		if err := http.ListenAndServe(*listenAddress, nil); err != nil {
			sigCh <- os.Interrupt
			log.Fatal(err)
		}
	}()

	log.Infof("Listening on %s", *listenAddress)

	// Force the first read
	go pollWave(wave, exp, *retries)

	// Listen to channels
	for {
		select {
		case <-sigCh:
			log.Info("Received signal")
			err := wave.Disconnect()
			if err != nil {
				log.Errorf("failed to disconnect BLE adapter: %v", err)
			}
			os.Exit(0)
		case <-tickCh:
			pollWave(wave, exp, *retries)
		}
	}
}

func pollWave(wave *Wave, exp *Exporter, retries int) {
	var currentReadValues *CurrentValues
	var err error

	currentReadValues, err = wave.Read()
	if err != nil {
		if retries > 1 {
			log.Errorf("failed to read values from Wave, retring %d times: %v", retries, err)
			pollWave(wave, exp, retries-1)
		} else {
			log.Errorf("failed to read values from Wave: %v", err)
		}

		return
	}

	exp.Collect(currentReadValues)
}
