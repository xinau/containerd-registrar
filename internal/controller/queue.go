package controller

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type QueueEventHandler struct {
	workqueue.Interface
}

var queueKeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

func NewQueueEventHandler() QueueEventHandler {
	return QueueEventHandler{workqueue.New()}
}

func (qeh *QueueEventHandler) GetEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, _ := queueKeyFunc(obj)
			qeh.Add(key)
		},
		UpdateFunc: func(_, obj interface{}) {
			key, _ := queueKeyFunc(obj)
			qeh.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, _ := queueKeyFunc(obj)
			qeh.Done(key)
		},
	}
}

func (qeh *QueueEventHandler) ProcessNextKey(ctx context.Context, process func(context.Context, interface{}) bool) bool {
	if ctx.Err() != nil {
		return false
	}

	key, quit := qeh.Get()
	if quit {
		return false
	}

	defer qeh.Done(key)

	return process(ctx, key)
}
