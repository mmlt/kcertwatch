package kclient

import (
	"fmt"
	"github.com/mmlt/kcertscan/internal/testdata"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"strings"
	"testing"
	"time"
)


// Create a Secret.
func Test_Create(t *testing.T) {
	// Setup.
	stopCh := make(chan struct{})
	defer close(stopCh)
	kc, client := startTestController(t, stopCh)
	defer prometheus.Unregister(kc.gaugeVec)

	// Create secret.
	_, err := client.CoreV1().Secrets("default").Create(newTestSecret("testsecret", testdata.Certs))
	assert.NoError(t, err, "secret create")
	err = waitForChanges(kc, 3)
	assert.NoError(t, err)

	// Check.
	assert.Equal(t, 3, kc.addTally)
	assert.Equal(t, 0, kc.deleteTally)

	metrics := []string{"kcertwatch_cert_expire_time_seconds"}
	want := `
# HELP kcertwatch_cert_expire_time_seconds NotAfter time of certificate in seconds.
# TYPE kcertwatch_cert_expire_time_seconds gauge
kcertwatch_cert_expire_time_seconds{field="certPEM",name="testsecret",namespace="default"} 1.4013216e+09
kcertwatch_cert_expire_time_seconds{field="rootPEM",name="testsecret",namespace="default"} 1.428160555e+09
kcertwatch_cert_expire_time_seconds{field="tls.crt",name="testsecret",namespace="default"} 1.506300437e+09
`
	err = testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(want), metrics...)
	assert.NoError(t, err)
}

// CreateThenDelete a Secret.
func Test_CreateThenDelete(t *testing.T) {
	// Setup.
	stopCh := make(chan struct{})
	defer close(stopCh)
	kc, client := startTestController(t, stopCh)
	defer prometheus.Unregister(kc.gaugeVec)

	// Create secret.
	_, err := client.CoreV1().Secrets("default").Create(newTestSecret("testsecret", testdata.Certs))
	assert.NoError(t, err, "secret create")
	err = waitForChanges(kc, 3)
	assert.NoError(t, err)
	// Delete secret.
	err = client.CoreV1().Secrets("default").Delete("testsecret", nil)
	assert.NoError(t, err, "secret delete")
	err = waitForChanges(kc, 0)
	assert.NoError(t, err)

	// Check
	assert.Equal(t, 3, kc.addTally, "addTally")
	assert.Equal(t, 3, kc.deleteTally, "addTally")

	metrics := []string{"kcertwatch_cert_expire_time_seconds"}
	want := `
`
	err = testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(want), metrics...)
	assert.NoError(t, err)
}

// CreateThenUpdate a Secret.
func Test_CreateThenUpdate(t *testing.T) {
	// Setup.
	stopCh := make(chan struct{})
	defer close(stopCh)
	kc, client := startTestController(t, stopCh)
	defer prometheus.Unregister(kc.gaugeVec)

	// Create secret.
	_, err := client.CoreV1().Secrets("default").Create(newTestSecret("testsecret", testdata.Certs))
	assert.NoError(t, err, "secret create")
	err = waitForChanges(kc, 3)
	assert.NoError(t, err)
	// Update secret (remove certPEM and pubPEM, keep rootPEM)
	_, err = client.CoreV1().Secrets("default").Update(newTestSecret("testsecret", map[string][]byte{"rootPEM": testdata.Certs["rootPEM"]}))
	assert.NoError(t, err, "secret delete")
	err = waitForChanges(kc, 1)
	assert.NoError(t, err)

	// Check
	assert.Equal(t, 3+1, kc.addTally, "addTally")
	assert.Equal(t, 3, kc.deleteTally, "addTally")

	metrics := []string{"kcertwatch_cert_expire_time_seconds"}
	want := `
# HELP kcertwatch_cert_expire_time_seconds NotAfter time of certificate in seconds.
# TYPE kcertwatch_cert_expire_time_seconds gauge
kcertwatch_cert_expire_time_seconds{field="rootPEM",name="testsecret",namespace="default"} 1.428160555e+09
`
	err = testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(want), metrics...)
	assert.NoError(t, err)
}

// Create a Secret then expire it so it gets GC'd.
func Test_CreateThenGC(t *testing.T) {
	// Setup.
	stopCh := make(chan struct{})
	defer close(stopCh)
	kc, client := startTestController(t, stopCh)
	defer prometheus.Unregister(kc.gaugeVec)

	// Create secret.
	client.CoreV1().Secrets("default").Create(newTestSecret("testsecret", testdata.Certs))
	err := waitForChanges(kc, 3)
	assert.NoError(t, err)

	// advance time by 3 resync periods to simulate a Secret that has been deleted without getting a delete event.
	TimeNow = func () time.Time {return time.Now().Add(3*ResyncPeriod)}

	// GC Secrets older then 2 periods.
	kc.secretGC(2*ResyncPeriod)

	// Check
	assert.Equal(t, 3, kc.addTally)
	assert.Equal(t, 0, kc.deleteTally)
	assert.Equal(t, 3, kc.gcDeleteTally)

	metrics := []string{"kcertwatch_cert_expire_time_seconds"}
	want := `
`
	err = testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(want), metrics...)
	assert.NoError(t, err)
}



/*** Helpers ******************************************************************/

func startTestController(t *testing.T, stopCh chan struct{}) (*kclient, kubernetes.Interface) {
	// Create controller.
	kc, client, sharedInformers, err := newTestController()
	if err != nil {
		t.Fatalf("error creating controller: %v", err)
	}

	// Start the controller.
	sharedInformers.Start(stopCh)

	return kc, client
}

// WaitForChanges waits till expectedCerts are detected.
func waitForChanges(kc *kclient, expectedCerts int) error {
	var n int
	err := wait.Poll(10*time.Millisecond, time.Second, func() (done bool, err error) {
		kc.RLock()
		defer kc.RUnlock()
		n = len(kc.lastSeen)
		done = n == expectedCerts
		return
	})
	if err != nil {
		return fmt.Errorf("waiting for %d certs: %v", expectedCerts, err)
	}

	return nil
}

func newTestController(initialObjects ...runtime.Object) (*kclient, kubernetes.Interface, informers.SharedInformerFactory, error) {
	client := kubernetesfake.NewSimpleClientset(initialObjects...)
	sharedInformers := informers.NewSharedInformerFactory(client, 10*time.Minute)

	// Create a controller that's instrumented for testing.
	kc := New(client, sharedInformers)
	kc.recorder = record.NewFakeRecorder(100)

	alwaysReady := func() bool { return true }
	kc.secretStoreSynced = alwaysReady

	return kc, client, sharedInformers, nil
}

// NewTestSecret returns a Secret with data.
// Assumes data values are not yet base64 encoded.
func newTestSecret(name string, data map[string][]byte) *corev1.Secret {
	//d := make(map[string][]byte, len(data))
	//for k,v := range data {
	//	d[k] = []byte(base64.StdEncoding.EncodeToString(v))
	//}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeOpaque, //TODO create tls?
		Data: data,
	}
}
