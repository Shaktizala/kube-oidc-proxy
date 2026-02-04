package crd

import (
	"fmt"
	"sync"
	"time"

	"github.com/Improwised/kube-oidc-proxy/pkg/cluster"
	"github.com/Improwised/kube-oidc-proxy/pkg/logger"
	"github.com/Improwised/kube-oidc-proxy/pkg/util"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type CAPIRbacWatcher struct {
	CAPIClusterRoleInformer        cache.SharedIndexInformer
	CAPIRoleInformer               cache.SharedIndexInformer
	CAPIClusterRoleBindingInformer cache.SharedIndexInformer
	CAPIRoleBindingInformer        cache.SharedIndexInformer
	clusters                       []*cluster.Cluster
	initialProcessingComplete      bool
	mu                             sync.RWMutex
}

func NewCAPIRbacWatcher(clusters []*cluster.Cluster) (*CAPIRbacWatcher, error) {

	clusterConfig, err := util.BuildConfiguration()
	if err != nil {
		return &CAPIRbacWatcher{}, err
	}

	clusterClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		return &CAPIRbacWatcher{}, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(clusterClient,
		time.Minute, "", nil)

	capiRoleInformer := factory.ForResource(CAPIRoleGVR).Informer()
	capiRoleBindingInformer := factory.ForResource(CAPIRoleBindingGVR).Informer()
	capiClusterRoleInformer := factory.ForResource(CAPIClusterRoleGVR).Informer()
	capiClusterRoleBindingInformer := factory.ForResource(CAPIClusterRoleBindingGVR).Informer()

	watcher := &CAPIRbacWatcher{
		CAPIRoleInformer:               capiRoleInformer,
		CAPIClusterRoleInformer:        capiClusterRoleInformer,
		CAPIRoleBindingInformer:        capiRoleBindingInformer,
		CAPIClusterRoleBindingInformer: capiClusterRoleBindingInformer,
		clusters:                       clusters,
	}

	watcher.RegisterEventHandlers()

	return watcher, nil
}

// Start the informers
func (w *CAPIRbacWatcher) Start(stopCh <-chan struct{}) {
	go w.CAPIRoleInformer.Run(stopCh)
	go w.CAPIClusterRoleInformer.Run(stopCh)
	go w.CAPIRoleBindingInformer.Run(stopCh)
	go w.CAPIClusterRoleBindingInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh,
		w.CAPIRoleInformer.HasSynced,
		w.CAPIClusterRoleInformer.HasSynced,
		w.CAPIRoleBindingInformer.HasSynced,
		w.CAPIClusterRoleBindingInformer.HasSynced,
	)
}

func (w *CAPIRbacWatcher) RegisterEventHandlers() {
	// Register event handlers for CAPIRole
	w.CAPIRoleInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !w.initialProcessingComplete {
				logger.Logger.Debug("Skipping CAPIRole add event during initial processing")
				return
			}
			capiRole, err := ConvertUnstructured[CAPIRole](obj)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIRole", zap.Error(err))
				return
			}
			w.ProcessCAPIRole(capiRole)
			w.RebuildAllAuthorizers()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCapiRole, err := ConvertUnstructured[CAPIRole](oldObj)
			if err != nil {
				logger.Logger.Error("Failed to convert old CAPIRole", zap.Error(err))
				return
			}
			newCapiRole, err := ConvertUnstructured[CAPIRole](newObj)
			if err != nil {
				logger.Logger.Error("Failed to convert new CAPIRole", zap.Error(err))
				return
			}
			if oldCapiRole.ResourceVersion == newCapiRole.ResourceVersion {
				logger.Logger.Debug("ResourceVersion is the same, skipping update")
				return
			}
			w.DeleteCAPIRole(oldCapiRole)
			w.ProcessCAPIRole(newCapiRole)
			w.RebuildAllAuthorizers()
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Logger.Error("Unexpected type in DeleteFunc for CAPIRole",
					zap.String("got", fmt.Sprintf("%T", obj)))
				return
			}
			capiRole, err := ConvertUnstructured[CAPIRole](u)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIRole during deletion", zap.Error(err))
				return
			}
			w.DeleteCAPIRole(capiRole)
			w.RebuildAllAuthorizers()
		},
	})

	// Register event handlers for CAPIClusterRole
	w.CAPIClusterRoleInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !w.initialProcessingComplete {
				logger.Logger.Debug("Skipping CAPIClusterRole add event during initial processing")
				return
			}
			capiClusterRole, err := ConvertUnstructured[CAPIClusterRole](obj)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIClusterRole", zap.Error(err))
				return
			}
			w.ProcessCAPIClusterRole(capiClusterRole)
			w.RebuildAllAuthorizers()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCapiClusterRole, err := ConvertUnstructured[CAPIClusterRole](oldObj)
			if err != nil {
				logger.Logger.Error("Failed to convert old CAPIClusterRole", zap.Error(err))
				return
			}
			newCapiClusterRole, err := ConvertUnstructured[CAPIClusterRole](newObj)
			if err != nil {
				logger.Logger.Error("Failed to convert new CAPIClusterRole", zap.Error(err))
				return
			}
			if oldCapiClusterRole.ResourceVersion == newCapiClusterRole.ResourceVersion {
				logger.Logger.Debug("ResourceVersion is the same, skipping update")
				return
			}
			w.DeleteCAPIClusterRole(oldCapiClusterRole)
			w.ProcessCAPIClusterRole(newCapiClusterRole)
			w.RebuildAllAuthorizers()
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Logger.Error("Unexpected type in DeleteFunc for CAPIClusterRole",
					zap.String("got", fmt.Sprintf("%T", obj)))
				return
			}
			capiClusterRole, err := ConvertUnstructured[CAPIClusterRole](u)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIClusterRole during deletion", zap.Error(err))
				return
			}
			w.DeleteCAPIClusterRole(capiClusterRole)
			w.RebuildAllAuthorizers()
		},
	})

	// Register event handlers for CAPIRoleBinding
	w.CAPIRoleBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !w.initialProcessingComplete {
				logger.Logger.Debug("Skipping CAPIRoleBinding add event during initial processing")
				return
			}
			capiRoleBinding, err := ConvertUnstructured[CAPIRoleBinding](obj)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIRoleBinding", zap.Error(err))
				return
			}
			w.ProcessCAPIRoleBinding(capiRoleBinding)
			w.RebuildAllAuthorizers()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCapiRoleBinding, err := ConvertUnstructured[CAPIRoleBinding](oldObj)
			if err != nil {
				logger.Logger.Error("Failed to convert old CAPIRoleBinding", zap.Error(err))
				return
			}
			newCapiRoleBinding, err := ConvertUnstructured[CAPIRoleBinding](newObj)
			if err != nil {
				logger.Logger.Error("Failed to convert new CAPIRoleBinding", zap.Error(err))
				return
			}
			if oldCapiRoleBinding.ResourceVersion == newCapiRoleBinding.ResourceVersion {
				logger.Logger.Debug("ResourceVersion is the same, skipping update")
				return
			}
			w.DeleteCAPIRoleBinding(oldCapiRoleBinding)
			w.ProcessCAPIRoleBinding(newCapiRoleBinding)
			w.RebuildAllAuthorizers()
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Logger.Error("Unexpected type in DeleteFunc for CAPIRoleBinding",
					zap.String("got", fmt.Sprintf("%T", obj)))
				return
			}
			capiRoleBinding, err := ConvertUnstructured[CAPIRoleBinding](u)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIRoleBinding during deletion", zap.Error(err))
				return
			}
			w.DeleteCAPIRoleBinding(capiRoleBinding)
			w.RebuildAllAuthorizers()
		},
	})

	// Register event handlers for CAPIClusterRoleBinding
	w.CAPIClusterRoleBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !w.initialProcessingComplete {
				logger.Logger.Debug("Skipping CAPIClusterRoleBinding add event during initial processing")
				return
			}
			capiClusterRoleBinding, err := ConvertUnstructured[CAPIClusterRoleBinding](obj)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIClusterRoleBinding", zap.Error(err))
				return
			}
			w.ProcessCAPIClusterRoleBinding(capiClusterRoleBinding)
			w.RebuildAllAuthorizers()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCapiClusterRoleBinding, err := ConvertUnstructured[CAPIClusterRoleBinding](oldObj)
			if err != nil {
				logger.Logger.Error("Failed to convert old CAPIClusterRoleBinding", zap.Error(err))
				return
			}
			newCapiClusterRoleBinding, err := ConvertUnstructured[CAPIClusterRoleBinding](newObj)
			if err != nil {
				logger.Logger.Error("Failed to convert new CAPIClusterRoleBinding", zap.Error(err))
				return
			}
			if oldCapiClusterRoleBinding.ResourceVersion == newCapiClusterRoleBinding.ResourceVersion {
				logger.Logger.Debug("ResourceVersion is the same, skipping update")
				return
			}
			w.DeleteCAPIClusterRoleBinding(oldCapiClusterRoleBinding)
			w.ProcessCAPIClusterRoleBinding(newCapiClusterRoleBinding)
			w.RebuildAllAuthorizers()
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Logger.Error("Unexpected type in DeleteFunc for CAPIClusterRoleBinding",
					zap.String("got", fmt.Sprintf("%T", obj)))
				return
			}
			capiClusterRoleBinding, err := ConvertUnstructured[CAPIClusterRoleBinding](u)
			if err != nil {
				logger.Logger.Error("Failed to convert CAPIClusterRoleBinding during deletion", zap.Error(err))
				return
			}
			w.DeleteCAPIClusterRoleBinding(capiClusterRoleBinding)
			w.RebuildAllAuthorizers()
		},
	})
}

func (w *CAPIRbacWatcher) UpdateClusters(clusters []*cluster.Cluster) {
	w.clusters = clusters
}
