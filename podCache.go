package container_info


import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	"time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informerCore "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	podHostField = "spec.nodeName"
)

//PodCache contains a podInformer of pod
type PodCache struct {
	podInformer informerCore.PodInformer
}


//NewPodCache creates a new podCache
func NewPodCache(client kubernetes.Interface, hostName string) *PodCache {
	podCache := new(PodCache)

	factory := informers.NewSharedInformerFactoryWithOptions(client, time.Minute, informers.WithTweakListOptions(func(options *metav1.
	ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector(podHostField, hostName).String()
	}))
	podCache.podInformer = factory.Core().V1().Pods()

	ch := make(chan struct{})
	go podCache.podInformer.Informer().Run(ch)

	for !podCache.podInformer.Informer().HasSynced() {
		time.Sleep(time.Second)
	}
	klog.V(2).Infof("Pod cache is running")

	return podCache
}

//OnAdd is a callback function for podInformer, do nothing for now.
func (p *PodCache) OnAdd(obj interface{}) {}

//OnUpdate is a callback function for podInformer, do nothing for now.
func (p *PodCache) OnUpdate(oldObj, newObj interface{}) {}

//OnDelete is a callback function for podInformer, do nothing for now.
func (p *PodCache) OnDelete(obj interface{}) {}


func (p *PodCache) GetPod(namespace, name string) (*v1.Pod, error) {
	pod, err := p.podInformer.Lister().Pods(namespace).Get(name)
	if err != nil {
		return nil, err
	}

	if podIsTerminated(pod) {
		return nil, fmt.Errorf("terminated pod")
	}

	if !IsGPURequiredPod(pod) {
		return nil, fmt.Errorf("no gpu pod")
	}

	return pod, nil
}