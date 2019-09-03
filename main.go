package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/mmlt/kcertwatch/internal/kclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"sync"
)

var (
	// Version as set during build.
	Version string

	k8sApi = flag.String("k8s-api", "",
		`URL of Kubernetes API server or "" when running in-cluster`)

	namespace = flag.String("namespace", "",
		`Namespace to watch, if omitted all namespaces are watched.`)

	promAddrs = flag.String("prom-addrs", ":9102",
		`The Prometheus endpoint address.`)
)

func init() {
	// Create Prometheus counters for number of klog'd info, warning and error lines.
	logged_errors := prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Subsystem: kclient.OperatorName,
			Name:      "logged_errors",
			Help:      "Number of logged errors.",
		},
		func() float64 {
			return float64(glog.Stats.Error.Lines())
		})

	prometheus.MustRegister(logged_errors)

	logged_warnings := prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Subsystem: kclient.OperatorName,
			Name:      "logged_warnings",
			Help:      "Number of logged warnings.",
		},
		func() float64 {
			return float64(glog.Stats.Warning.Lines())
		})

	prometheus.MustRegister(logged_warnings)

	logged_info := prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Subsystem: kclient.OperatorName,
			Name:      "logged_info",
			Help:      "Number of logged info.",
		},
		func() float64 {
			return float64(glog.Stats.Info.Lines())
		})

	prometheus.MustRegister(logged_info)
}

func main() {
	_ = flag.Set("alsologtostderr", "true")
	flag.Parse()

	s := fmt.Sprintf("Start %s %s", kclient.OperatorName, Version)
	flag.VisitAll(func(f *flag.Flag) {
		s = fmt.Sprintf("%s %s=%q", s, f.Name, f.Value)
	})
	glog.Info(s)

	// Start components
	config, err := clientcmd.BuildConfigFromFlags(*k8sApi, "")
	if err != nil {
		glog.Exit(err)
	}

	kubeClient := kubernetes.NewForConfigOrDie(config)
	sharedInformers := informers.NewFilteredSharedInformerFactory(kubeClient, kclient.ResyncPeriod, *namespace, nil)

	// Create kube API Server client.
	kc := kclient.New(kubeClient, sharedInformers)

	// Start the instances.
	stop := make(chan struct{})
	sharedInformers.Start(stop)
	wg := &sync.WaitGroup{} // GO routines should add themselves
	go kc.Run(stop, wg)

	// Start prometheus endpoint
	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(*promAddrs, nil)
	if err != http.ErrServerClosed {
		glog.Error(err)
	}

	glog.Info("Shutting down.")
	close(stop)
	wg.Wait()
}
