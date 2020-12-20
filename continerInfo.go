package container_info

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/qos"
	criapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"os"
	"path/filepath"
	"strings"
	"time"
	"k8s.io/klog"
)

type ContainerInfoInterface interface {
	// Get pids in the given container id
	GetPidsInContainers(containerID string) ([]int, error)
	// InspectContainer returns the container information by the given name
	InspectContainer(containerID string) (*criapi.ContainerStatus, error)
}

type containerInfoManager struct {
	criapi.RuntimeServiceClient
	podCache *PodCache
	requestTimeout time.Duration

}

func NewContainerRuntimeManager(cgroupDriver, endpoint string, requestTimeout time.Duration) (*containerInfoManager, error) {
	dialOptions := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDialer(UnixDial), grpc.WithBlock(), grpc.WithTimeout(time.Second * 5)}
	conn, err := grpc.Dial(endpoint, dialOptions...)
	if err != nil {
		return nil, err
	}

	client := criapi.NewRuntimeServiceClient(conn)


	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	k8sclient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		panic(err.Error())
	}

	m := &containerInfoManager{
		RuntimeServiceClient:   client,
		podCache:               NewPodCache(k8sclient, "fuck"),
		requestTimeout: 		requestTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.requestTimeout)
	defer cancel()
	resp, err := client.Version(ctx, &criapi.VersionRequest{Version: "0.1.0"})
	if err != nil {
		return nil, err
	}

	klog.V(2).Infof("Container runtime is %s", resp.RuntimeName)

	return m, nil
}

func (m *containerInfoManager) GetPidsInContainers(containerID string) ([]int, error) {
	req := &criapi.ContainerStatusRequest{
		ContainerId: containerID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.requestTimeout)
	defer cancel()

	resp, err := m.ContainerStatus(ctx, req)
	if err != nil {
		klog.Errorf("can't get container %s status, %v", containerID, err)
		return nil, err
	}

	ns := resp.Status.Labels[PodNamespaceLabelKey]
	podName := resp.Status.Labels[PodNameLabelKey]

	pod, err := m.podCache.GetPod(ns, podName)
	if err != nil {
		klog.Errorf("can't get pod %s/%s, %v", ns, podName, err)
		return nil, err
	}

	cgroupPath, err := m.getCgroupName(pod, containerID)
	if err != nil {
		klog.Errorf("can't get cgroup parent, %v", err)
		return nil, err
	}

	pids := make([]int, 0)
	baseDir := filepath.Clean(filepath.Join(CGROUP_BASE, cgroupPath))
	filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Name() != CGROUP_PROCS {
			return nil
		}

		p, err := readProcsFile(path)
		if err == nil {
			pids = append(pids, p...)
		}

		return nil
	})

	return pids, nil
}

func (m *containerInfoManager) InspectContainer(containerID string) (*criapi.ContainerStatus, error) {
	req := &criapi.ContainerStatusRequest{
		ContainerId: containerID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.requestTimeout)
	defer cancel()

	resp, err := m.ContainerStatus(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Status, nil
}


func (m *containerInfoManager) getCgroupName(pod *v1.Pod, containerID string) (string, error) {
	podQos := qos.GetPodQOS(pod)

	var parentContainer CgroupName
	switch podQos {
	case v1.PodQOSGuaranteed:
		parentContainer = NewCgroupName(containerRoot)
	case v1.PodQOSBurstable:
		parentContainer = NewCgroupName(containerRoot, strings.ToLower(string(v1.PodQOSBurstable)))
	case v1.PodQOSBestEffort:
		parentContainer = NewCgroupName(containerRoot, strings.ToLower(string(v1.PodQOSBestEffort)))
	}

	podContainer := PodCgroupNamePrefix + string(pod.UID)
	cgroupName := NewCgroupName(parentContainer, podContainer)

	return fmt.Sprintf("%s/%s-%s.scope", cgroupName.ToSystemd(), "crio-conmon", containerID), nil

}


