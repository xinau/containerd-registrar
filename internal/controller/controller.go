package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	nodeStateAnnotation = "node.containerd-registrar.io/node-state"

	nodeNameIndexer = "node-name-indexer"
)

func getObjectFromStoreByKey(store cache.Store, key string) (interface{}, bool) {
	obj, exists, err := store.GetByKey(key)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false
	}

	if !exists || obj == nil {
		return nil, false
	}

	return obj, true
}

func hasTaintWithKey(node *corev1.Node, key string) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

func indexByNodeName(obj interface{}) ([]string, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("obj isn't of type *corev1.Pod, got: %T", obj)
	}
	return []string{pod.Spec.NodeName}, nil
}

func withLabelSelector(ls string) informers.SharedInformerOption {
	return informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = ls
	})
}

type Config struct {
	AgentNodeTaint    string
	AgentPodNamespace string
	AgentPodLabels    string
	ResyncInterval    time.Duration
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

func (mgr *Manager) isAgentRunning(nodeName string) bool {
	pods, err := mgr.podInformer.GetIndexer().ByIndex(nodeNameIndexer, nodeName)
	if err != nil {
		return false
	}

	for _, obj := range pods {
		pod := obj.(*corev1.Pod)
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady {
				return true
			}
		}
	}

	return false
}

type patch struct {
	OP    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

var escapePatchPath = strings.NewReplacer(
	"~", "~0",
	"/", "~1",
).Replace

func (mgr *Manager) markNodeAsReady(ctx context.Context, node *corev1.Node) error {
	var taints []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != mgr.cfg.AgentNodeTaint {
			taints = append(taints, taint)
		}
	}

	patches := []patch{{
		OP:    "replace",
		Path:  fmt.Sprintf("/metadata/annotations/%s", escapePatchPath(nodeStateAnnotation)),
		Value: "complete",
	}, {
		OP:    "replace",
		Path:  "/spec/taints",
		Value: taints,
	}}

	payload, err := json.Marshal(patches)
	if err != nil {
		return err
	}

	logrus.WithField("node", node.Name).Debug("submitting node patch")
	_, err = mgr.client.CoreV1().Nodes().Patch(ctx, node.Name, apitypes.JSONPatchType, payload, metav1.PatchOptions{})
	return err
}

func (mgr *Manager) checkAndMarkNode(ctx context.Context, nodeName string) {
	obj, exists := getObjectFromStoreByKey(mgr.nodeInformer.GetStore(), nodeName)
	if !exists {
		return
	}

	node := obj.(*corev1.Node)

	if hasTaintWithKey(node, mgr.cfg.AgentNodeTaint) && mgr.isAgentRunning(node.Name) {
		if err := mgr.markNodeAsReady(ctx, node); err != nil {
			logrus.WithField("node", nodeName).WithError(err).Warn("failed marking node as ready")
		} else {
			logrus.WithField("node", nodeName).Debug("node isn't ready")
		}
		logrus.WithField("node", nodeName).Info("marked node as ready")
		return
	} else {
		logrus.WithField("node", nodeName).Debug("node without agent taint")
	}

	return
}

func (mgr *Manager) processNextPodItem(ctx context.Context, key interface{}) bool {
	obj, exists := getObjectFromStoreByKey(mgr.podInformer.GetStore(), key.(string))
	if !exists {
		return ctx.Err() == nil
	}

	pod := obj.(*corev1.Pod)
	mgr.checkAndMarkNode(ctx, pod.Spec.NodeName)

	return ctx.Err() == nil
}

func (mgr *Manager) processNextNodeItem(ctx context.Context, key interface{}) bool {
	mgr.checkAndMarkNode(ctx, key.(string))
	return ctx.Err() == nil
}

func (mgr *Manager) watchPods(ctx context.Context) {
	factory := informers.NewSharedInformerFactoryWithOptions(mgr.client, mgr.cfg.ResyncInterval,
		informers.WithNamespace(mgr.cfg.AgentPodNamespace),
		withLabelSelector(mgr.cfg.AgentPodLabels),
	)
	mgr.podInformer = factory.Core().V1().Pods().Informer()

	queue := NewQueueEventHandler()
	mgr.podInformer.AddEventHandler(queue.GetEventHandler())
	mgr.podInformer.AddIndexers(cache.Indexers{nodeNameIndexer: indexByNodeName})

	go mgr.podInformer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), mgr.podInformer.HasSynced)

	for queue.ProcessNextKey(ctx, mgr.processNextPodItem) {
	}
}

func (mgr *Manager) watchNodes(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(mgr.client, mgr.cfg.ResyncInterval)
	mgr.nodeInformer = factory.Core().V1().Nodes().Informer()

	queue := NewQueueEventHandler()
	mgr.nodeInformer.AddEventHandler(queue.GetEventHandler())

	go mgr.nodeInformer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), mgr.nodeInformer.HasSynced)

	for queue.ProcessNextKey(ctx, mgr.processNextNodeItem) {
	}
}

func (mgr *Manager) Run(ctx context.Context) error {
	go mgr.watchNodes(ctx)
	mgr.watchPods(ctx)

	return ctx.Err()
}
