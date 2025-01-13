// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package util

import "github.com/sapcc/kube-parrot/pkg/forked/workqueue"

type DirtyReconcilerInterface interface {
	Interface
	Dirty()
}

type dirtyReconciler struct {
	Type
}

func NewNamedDirtyReconciler(name string, reconcileFunc func() error) DirtyReconcilerInterface {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name)

	return &dirtyReconciler{
		Type{queue, reconcileFunc},
	}
}

func (c *dirtyReconciler) Dirty() {
	c.queue.AddRateLimited("dirty")
}
