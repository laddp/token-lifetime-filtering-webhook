package informer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tokenv1alpha1 "github.com/pladd/token-lifetime-filtering-webhook/api/v1alpha1"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

var policyGVR = schema.GroupVersionResource{
	Group:    tokenv1alpha1.Group,
	Version:  tokenv1alpha1.Version,
	Resource: "tokenlifetimepolicies",
}

// Run starts an informer that keeps store updated with TokenLifetimePolicy objects.
// It returns a HasSynced function.
func Run(ctx context.Context, dyn dynamic.Interface, store *policy.Store) (cache.InformerSynced, error) {
	client := dyn.Resource(policyGVR)

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.Watch(ctx, options)
		},
	}

	_, controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: lw,
		ObjectType:    &unstructured.Unstructured{},
		ResyncPeriod:  10 * time.Minute,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				upsert(store, obj)
			},
			UpdateFunc: func(_, newObj interface{}) {
				upsert(store, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				u, err := toUnstructured(obj)
				if err != nil {
					slog.Error("delete handler", "err", err)
					return
				}
				store.Delete(u.GetName())
			},
		},
	})

	go controller.Run(ctx.Done())
	return controller.HasSynced, nil
}

func upsert(store *policy.Store, obj interface{}) {
	u, err := toUnstructured(obj)
	if err != nil {
		slog.Error("upsert", "err", err)
		return
	}
	pol, err := unstructuredToPolicy(u)
	if err != nil {
		slog.Error("convert TokenLifetimePolicy", "err", err)
		return
	}
	store.Upsert(pol)
}

func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		return t, nil
	case cache.DeletedFinalStateUnknown:
		if u, ok := t.Obj.(*unstructured.Unstructured); ok {
			return u, nil
		}
		return nil, fmt.Errorf("unexpected DeletedFinalStateUnknown type %T", t.Obj)
	default:
		return nil, fmt.Errorf("unexpected object type %T", obj)
	}
}

func unstructuredToPolicy(u *unstructured.Unstructured) (*tokenv1alpha1.TokenLifetimePolicy, error) {
	pol := &tokenv1alpha1.TokenLifetimePolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pol); err != nil {
		return nil, err
	}
	return pol, nil
}
