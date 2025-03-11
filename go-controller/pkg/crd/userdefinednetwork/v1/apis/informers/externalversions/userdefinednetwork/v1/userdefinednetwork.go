/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	context "context"
	time "time"

	crduserdefinednetworkv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	versioned "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1/apis/clientset/versioned"
	internalinterfaces "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1/apis/informers/externalversions/internalinterfaces"
	userdefinednetworkv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1/apis/listers/userdefinednetwork/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// UserDefinedNetworkInformer provides access to a shared informer and lister for
// UserDefinedNetworks.
type UserDefinedNetworkInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() userdefinednetworkv1.UserDefinedNetworkLister
}

type userDefinedNetworkInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewUserDefinedNetworkInformer constructs a new informer for UserDefinedNetwork type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewUserDefinedNetworkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredUserDefinedNetworkInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredUserDefinedNetworkInformer constructs a new informer for UserDefinedNetwork type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredUserDefinedNetworkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.K8sV1().UserDefinedNetworks(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.K8sV1().UserDefinedNetworks(namespace).Watch(context.TODO(), options)
			},
		},
		&crduserdefinednetworkv1.UserDefinedNetwork{},
		resyncPeriod,
		indexers,
	)
}

func (f *userDefinedNetworkInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredUserDefinedNetworkInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *userDefinedNetworkInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&crduserdefinednetworkv1.UserDefinedNetwork{}, f.defaultInformer)
}

func (f *userDefinedNetworkInformer) Lister() userdefinednetworkv1.UserDefinedNetworkLister {
	return userdefinednetworkv1.NewUserDefinedNetworkLister(f.Informer().GetIndexer())
}
