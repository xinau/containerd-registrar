package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Config struct {
	AgentLabels    string
	AgentNamespace string
	AgentNodeTaint string
	ResyncInterval time.Duration
}

type Manager struct {
	client *kubernetes.Clientset
	cfg    Config

	factory informers.SharedInformerFactory

	nodeInformer cache.SharedIndexInformer
	podInformer  cache.SharedIndexInformer
}

func NewManager(client *kubernetes.Clientset, cfg Config) *Manager {
	return &Manager{
		client: client,
		cfg:    cfg,
	}
}

func hasNodeTaint(node *corev1.Node, check string) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == check {
			return true
		}
	}
	return false
}

func hasPodCondition(pod *corev1.Pod, check corev1.PodConditionType) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return true
		}
	}
	return false
}

func (mgr *Manager) isAgentPodRunning(nodeName string) bool {
	pods, err := mgr.podInformer.GetIndexer().ByIndex(nodeNameIndexer, nodeName)
	if err != nil || len(pods) == 0 {
		return false
	}

	for _, obj := range pods {
		pod := obj.(*corev1.Pod)
		if hasPodCondition(pod, corev1.PodReady) && pod.Status.Phase == corev1.PodRunning {
			return true
		}
	}

	return false
}

func (mgr *Manager) removeAgentNodeTaint(ctx context.Context, node *corev1.Node) {
	var found bool
	var taints []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != mgr.cfg.AgentNodeTaint {
			taints = append(taints, taint)
		} else {
			found = true
		}
	}

	if !found {
		return
	}

	result := node.DeepCopy()
	result.Spec.Taints = taints

	logfields := logrus.Fields{"node": node.GetName(), "taint": mgr.cfg.AgentNodeTaint}
	logrus.WithFields(logfields).Debug("removing agent taint from node")
	_, err := mgr.client.CoreV1().Nodes().Update(ctx, result, metav1.UpdateOptions{})
	if err != nil {
		logrus.WithFields(logfields).WithError(err).Warn("failed updating taints of node")
	}
	logrus.WithFields(logfields).Info("agent taint removed from node")
}

func (mgr *Manager) checkAndRemoveAgentNodeTaint(ctx context.Context, nodeName string) bool {
	obj, exists, err := mgr.nodeInformer.GetStore().GetByKey(nodeName)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return false
	}

	if !exists || obj == nil {
		return false
	}

	node := obj.(*corev1.Node)
	nodeName = node.GetName()

	logfields := logrus.Fields{"node": nodeName}
	if hasNodeTaint(node, mgr.cfg.AgentNodeTaint) {
		if mgr.isAgentPodRunning(nodeName) {
			logrus.WithField("node", nodeName).Debug("agent pod is up and running")
			mgr.removeAgentNodeTaint(ctx, node)
		} else {
			logrus.WithFields(logfields).Debug("agent pod isn't running")
		}
	} else {
		logrus.WithFields(logfields).Debug("node without agent taint")
	}

	return true
}

func (mgr *Manager) processNextAgentPodItem(ctx context.Context, queue workqueue.RateLimitingInterface) bool {
	key, quit := queue.Get()
	if quit {
		return false
	}

	defer queue.Done(key)

	obj, exists, err := mgr.podInformer.GetStore().GetByKey(key.(string))
	if err != nil && !k8sErrors.IsNotFound(err) {
		return true
	}

	if !exists || obj == nil {
		queue.Forget(key)
		return true
	}

	pod := obj.(*corev1.Pod)
	nodeName := pod.Spec.NodeName

	if !mgr.checkAndRemoveAgentNodeTaint(ctx, nodeName) {
		queue.Forget(key)
		return true
	}

	queue.Forget(key)
	return true
}

func nodeNameIndexFunc(obj interface{}) ([]string, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("obj isn't of type *corev1.Pod, got: %T", obj)
	}
	return []string{pod.Spec.NodeName}, nil
}

var (
	queueKeyFunc    = cache.DeletionHandlingMetaNamespaceKeyFunc
	nodeNameIndexer = "node-name-indexer"
)

func (mgr *Manager) watchAgentPods(ctx context.Context) {
	factory := informers.NewSharedInformerFactoryWithOptions(mgr.client, mgr.cfg.ResyncInterval,
		informers.WithNamespace(mgr.cfg.AgentNamespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = mgr.cfg.AgentLabels
		}))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod-queue")

	mgr.podInformer = factory.Core().V1().Pods().Informer()
	mgr.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, _ := queueKeyFunc(obj)
			queue.Add(key)
		},
		UpdateFunc: func(_, obj interface{}) {
			key, _ := queueKeyFunc(obj)
			queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, _ := queueKeyFunc(obj)
			queue.Done(key)
		},
	})
	mgr.podInformer.AddIndexers(cache.Indexers{nodeNameIndexer: nodeNameIndexFunc})

	go mgr.podInformer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), mgr.podInformer.HasSynced)

	for mgr.processNextAgentPodItem(ctx, queue) {
	}
}

func (mgr *Manager) watchNodes(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(mgr.client, mgr.cfg.ResyncInterval)
	mgr.nodeInformer = factory.Core().V1().Nodes().Informer()
	go mgr.nodeInformer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), mgr.nodeInformer.HasSynced)
}

func (mgr *Manager) Run(ctx context.Context) error {
	go mgr.watchNodes(ctx)
	mgr.watchAgentPods(ctx)

	return ctx.Err()
}
