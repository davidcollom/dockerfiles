package reaper

import (
	"context"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

type Options struct {
	Threshold time.Duration
	Delete    bool
	Namespace string
	Now       func() time.Time
}

type Reaper struct {
	client kubernetes.Interface
	logger *slog.Logger
	opts   Options
}

func New(client kubernetes.Interface, logger *slog.Logger, opts Options) *Reaper {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	return &Reaper{
		client: client,
		logger: logger,
		opts:   opts,
	}
}

func (r *Reaper) Run(ctx context.Context) error {
	namespace := r.opts.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	pods, err := r.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("status.phase", string(corev1.PodPending)).String(),
	})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if !r.shouldReap(pod) {
			continue
		}

		r.logger.Info(
			"stuck pod detected",
			"namespace", pod.Namespace,
			"name", pod.Name,
			"node", pod.Spec.NodeName,
			"age", r.opts.Now().Sub(pod.CreationTimestamp.Time).String(),
			"dry_run", !r.opts.Delete,
		)

		if !r.opts.Delete {
			continue
		}

		err := r.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		r.logger.Info("deleted stuck pod", "namespace", pod.Namespace, "name", pod.Name)
	}

	return nil
}

func (r *Reaper) shouldReap(pod corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}

	if len(pod.OwnerReferences) == 0 {
		return false
	}

	if r.opts.Now().Sub(pod.CreationTimestamp.Time) < r.opts.Threshold {
		return false
	}

	return hasStuckWaitingReason(pod)
}

func hasStuckWaitingReason(pod corev1.Pod) bool {
	for _, status := range pod.Status.InitContainerStatuses {
		if isStuckWaiting(status.State.Waiting) {
			return true
		}
	}

	for _, status := range pod.Status.ContainerStatuses {
		if isStuckWaiting(status.State.Waiting) {
			return true
		}
	}

	return false
}

func isStuckWaiting(waiting *corev1.ContainerStateWaiting) bool {
	if waiting == nil {
		return false
	}

	switch waiting.Reason {
	case "ContainerCreating", "PodInitializing":
		return true
	default:
		return false
	}
}
