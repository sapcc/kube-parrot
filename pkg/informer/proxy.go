package informer

import "k8s.io/client-go/1.5/tools/cache"

type KubeProxyIndexer struct {
	store cache.Indexer
}

func NewKubeProxyIndexer() KubeProxyIndexer {
	return KubeProxyIndexer{cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"kube-proxies": kubeProxiesIndexFunc})}
}

//func kubeProxiesIndexFunc(obj interface{}) ([]string, error) {
//  if pod, ok := obj.(*api.Pod); ok {
//    api.IsPodReady(pod)

//    return []string{""}, nil
//  }
//  return []string{""}, fmt.Errorf("object is not a pod: %v", obj)
//}
