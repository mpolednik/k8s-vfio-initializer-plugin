package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"kubevirt.io/kubevirt/pkg/kubecli"
	"kubevirt.io/kubevirt/pkg/service"
)

const (
	resyncPeriod = 30 * time.Second
)

type vfioInitializer struct {
}

func (init vfioInitializer) Run() {
	clientSet, err := kubecli.GetKubevirtClient()
	if err != nil {
		glog.Fatal(err)
	}

	restClient := clientSet.RestClient()

	lw := listWatchWithUninitialized(restClient, "pods", corev1.NamespaceAll, fields.Everything())

	_, controller := cache.NewInformer(lw, &corev1.Pod{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				err := initializeVirtualMachine(obj.(*corev1.Pod), &clientSet)
				if err != nil {
					glog.Error(err)
				}
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.V(3).Info("Received signal (SIGINT, SIGTERM), shutting down...")
	close(stop)
}

func initializeVirtualMachine(pod *corev1.Pod, clientset *kubecli.KubevirtClient) error {
	if initializers := pod.ObjectMeta.GetInitializers(); initializers != nil {
		pendingInitializers := initializers.Pending
		var initializerName string

		if initializerName == pendingInitializers[0].Name {
			glog.Infof("Initializing pod: %s", pod.Name)
		}
	}

	return nil
}

// Workaround for IncludeUninitialized setting from
// https://github.com/kelseyhightower/kubernetes-initializer-tutorial/blob/master/envoy-initializer/main.go.
// The interface{} argument is actually k8s client's cache.Getter, but we have to work around it since kubevirt can't still be imported by glide.
func listWatchWithUninitialized(c interface{}, resource string, namespace string, fieldSelector fields.Selector) *cache.ListWatch {
	lw := cache.NewListWatchFromClient(c.(cache.Getter), resource, namespace, fieldSelector)

	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return lw.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return lw.Watch(options)
		},
	}
}

func (init vfioInitializer) AddFlags() {
	return
}

func main() {
	app := &vfioInitializer{}
	service.Setup(app)
	app.Run()
}
