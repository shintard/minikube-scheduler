package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	apiserver "github.com/shintard/minikube-scheduler/api_server"
	"github.com/shintard/minikube-scheduler/config"
	pvcontroller "github.com/shintard/minikube-scheduler/pv_controller"
	"github.com/shintard/minikube-scheduler/scheduler"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func main() {
	if err := start(); err != nil {
		klog.Fatalf("failed with error on running scheduler: %s", err.Error())
	}
}

func start() error {
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	restClientCfg, apiShutDown, err := apiserver.StartAPIServer(cfg.EtcdURL)
	if err != nil {
		return err
	}
	defer apiShutDown()

	client := clientset.NewForConfigOrDie(restClientCfg)

	pvShutdown, err := pvcontroller.StartPersistentVolumeController(client)
	if err != nil {
		return err
	}
	defer pvShutdown()

	sched := scheduler.NewSchedulerService(client, restClientCfg)

	dsc, err := scheduler.DefaultSchedulerConfig()
	if err != nil {
		return err
	}

	if err := sched.StartScheduler(dsc); err != nil {
		return err
	}
	defer sched.ShutdownScheduler()

	err = scenario(client)
	if err != nil {
		return err
	}

	return nil
}

func scenario(client clientset.Interface) error {
	ctx := context.Background()

	// create node0 ~ node9, but all nodes are unschedulable
	for i := 0; i < 9; i++ {
		suffix := strconv.Itoa(i)
		_, err := client.CoreV1().Nodes().Create(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node" + suffix,
			},
			Spec: v1.NodeSpec{
				Unschedulable: true,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create node: %w", err)
		}
	}

	klog.Info("scenario: all nodes created")

	_, err := client.CoreV1().Pods("default").Create(ctx, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "container1",
					Image: "k8s.gcr.io/pause:3.5",
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	klog.Info("scenario: pod1 created")

	// wait to schedule
	time.Sleep(3 * time.Second)

	pod1, err := client.CoreV1().Pods("default").Get(ctx, "pod1", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get pod: %w", err)
	}
	if len(pod1.Spec.NodeName) != 0 {
		return fmt.Errorf("pod1 should not be bound yet")
	} else {
		klog.Info("pod1 is have not been bound yet.")
	}

	_, err = client.CoreV1().Nodes().Create(ctx, &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node10",
		},
		Spec: v1.NodeSpec{},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	klog.Info("node10 is created")

	// wait to schedule
	time.Sleep(5 * time.Second)

	pod1, err = client.CoreV1().Pods("default").Get(ctx, "pod1", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get pod: %w", err)
	}
	klog.Info("pod1 is bound to " + pod1.Spec.NodeName)

	return nil
}
