package kclient

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sync"
	"time"
)

// Controller watches Kubernetes ConfigMaps and Nodes resources.
type kclient struct {
	// Client for the k8s API Server.
	client kubernetes.Interface
	// Recorder to provide user feedback via Events.
	recorder record.EventRecorder
	// *StoreSynced func returns true when secretStore is in sync with the API Server.
	secretStoreSynced cache.InformerSynced

	// GaugeVec keeps track of cert expiry times.
	gaugeVec *prometheus.GaugeVec
	// LastSeen has the last time a secret/field has been added or updated.
	lastSeen map[string]time.Time
	// testing tallies.
	addTally, deleteTally, gcDeleteTally int

	sync.RWMutex
}

// New creates an API server client and subscribes to resource changes.
func New(kubeclientset kubernetes.Interface, sharedInformers informers.SharedInformerFactory) *kclient {
	// create event recorder
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: OperatorName})

	// create informers
	secretInformer := sharedInformers.Core().V1().Secrets()

	c := kclient{
		client:            kubeclientset,
		recorder:          recorder,
		secretStoreSynced: secretInformer.Informer().HasSynced,
		lastSeen:          map[string]time.Time{},
	}

	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.secretAdd,
			UpdateFunc: c.secretUpdate,
			DeleteFunc: c.secretDelete,
		},
	)

	c.gaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: OperatorName,
			Name:      "cert_expire_time_seconds",
			Help:      "NotAfter time of certificate in seconds.",
		},
		[]string{"secret_namespace", "secret_name", "secret_field"},
	)
	prometheus.MustRegister(c.gaugeVec)

	return &c
}

// Run kclient.
// go-routines should respect 'stop' channel and 'wg' when exiting.
func (kc *kclient) Run(stop chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(ResyncPeriod)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			kc.secretGC(2 * ResyncPeriod)
		}
	}
}

/***** API Instruction handlers ****************************************************/

// TimeNow allow one to manipulate time during testing.
var TimeNow = time.Now

// SecretAdd is called when a Secret is created.
func (kc *kclient) secretAdd(obj interface{}) {
	secret := obj.(*corev1.Secret)
	if !(secret.Type == corev1.SecretTypeOpaque || secret.Type == corev1.SecretTypeTLS) {
		return
	}

	glog.V(6).Infof("add %s", knn(secret))

	// search secret for certs
	exps, err := SearchExpiries(secret.Data)
	if err != nil {
		glog.Warningf("%s: %v", knn(secret), err)
		return
	}

	// add certs seconds to expiry to prometheus and record a last seen time.
	tn := TimeNow()
	for f, t := range exps {
		glog.V(2).Infof("add cert %s %s", knn(secret), f)

		kc.gaugeVec.WithLabelValues(secret.Namespace, secret.Name, f).Set(float64(t.Unix()))

		k := fmt.Sprintf("%s\n%s\n%s", secret.Namespace, secret.Name, f)
		kc.Lock()
		kc.lastSeen[k] = tn
		kc.addTally++
		kc.Unlock()
	}
}

// SecretDelete is called when a Secret is deleted.
func (kc *kclient) secretDelete(obj interface{}) {
	secret := obj.(*corev1.Secret)
	glog.V(6).Infof("delete %s", knn(secret))

	kc.RLock()
	defer kc.RUnlock()
	for k := range kc.lastSeen {
		var ns, n, f string
		fmt.Sscanf(k, "%s\n%s\n%s", &ns, &n, &f)
		if secret.Namespace == ns && secret.Name == n {
			glog.V(2).Infof("delete cert %s %s", knn(secret), f)
			ok := kc.gaugeVec.DeleteLabelValues(ns, n, f)
			if !ok {
				glog.Errorf("failed to delete metric with labels %s %s %s", ns, n, f)
			}
			kc.RUnlock()
			kc.Lock()
			delete(kc.lastSeen, k)
			kc.deleteTally++
			kc.Unlock()
			kc.RLock()
		}
	}
}

// SecretUpdate is called when a Secret fields are added, removed or changed.
func (kc *kclient) secretUpdate(old, obj interface{}) {
	secret := obj.(*corev1.Secret)
	glog.V(6).Infof("update %s", knn(secret))
	kc.secretDelete(old.(*corev1.Secret))
	kc.secretAdd(secret)
}

// SecretGC does a garbage collect of Secrets.
// Secrets are considered garbage when no event has been received from the API Server for 'age' time.
func (kc *kclient) secretGC(age time.Duration) {
	glog.V(6).Infof("secretGC")
	toOld := TimeNow().Add(-age)
	kc.RLock()
	defer kc.RUnlock()
	for k, t := range kc.lastSeen {
		if t.Before(toOld) {
			var ns, n, f string
			fmt.Sscanf(k, "%s\n%s\n%s", &ns, &n, &f)
			glog.V(2).Infof("delete cert %s/%s %s", ns, n, f)
			ok := kc.gaugeVec.DeleteLabelValues(ns, n, f)
			if !ok {
				glog.Errorf("failed to GC delete metric with labels %s %s %s", ns, n, f)
			}
			kc.RUnlock()
			kc.Lock()
			delete(kc.lastSeen, k)
			kc.gcDeleteTally++
			kc.Unlock()
			kc.RLock()
		}
	}
}

// KNN returns the Kind, Namespace, Name of 'in' formatted as string.
func knn(in *corev1.Secret) string {
	return fmt.Sprintf("secret %s/%s", in.Namespace, in.Name)
}
