/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package cluster

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/golang/glog"
	federation_v1alpha1 "k8s.io/kubernetes/federation/apis/federation/v1alpha1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	utilnet "k8s.io/kubernetes/pkg/util/net"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	UserAgentName           = "Cluster-Controller"
	KubeAPIQPS              = 20.0
	KubeAPIBurst            = 30
	KubeconfigSecretDataKey = "kubeconfig"
)

// This is to inject a different kubeconfigGetter in tests.
// We dont use the standard one which calls NewInCluster in tests to avoid having to setup service accounts and mount files with secret tokens.
var KubeconfigGetterForCluster = func(c *federation_v1alpha1.Cluster) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		// Get the namespace this is running in from the env variable.
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			return nil, fmt.Errorf("unexpected: POD_NAMESPACE env var returned empty string")
		}
		// Get a client to talk to the k8s apiserver, to fetch secrets from it.
		client, err := client.NewInCluster()
		if err != nil {
			return nil, fmt.Errorf("error in creating in-cluster client: %s", err)
		}
		secret, err := client.Secrets(namespace).Get(c.Spec.SecretRef.Name)
		if err != nil {
			return nil, fmt.Errorf("error in fetching secret: %s", err)
		}
		data, ok := secret.Data[KubeconfigSecretDataKey]
		if !ok {
			return nil, fmt.Errorf("secret does not have data with key: %s", KubeconfigSecretDataKey)
		}
		return clientcmd.Load(data)
	}
}

type ClusterClient struct {
	discoveryClient *discovery.DiscoveryClient
	kubeClient      *clientset.Clientset
}

func NewClusterClientSet(c *federation_v1alpha1.Cluster) (*ClusterClient, error) {
	var serverAddress string
	hostIP, err := utilnet.ChooseHostInterface()
	if err != nil {
		return nil, err
	}

	for _, item := range c.Spec.ServerAddressByClientCIDRs {
		_, cidrnet, err := net.ParseCIDR(item.ClientCIDR)
		if err != nil {
			return nil, err
		}
		myaddr := net.ParseIP(hostIP.String())
		if cidrnet.Contains(myaddr) == true {
			serverAddress = item.ServerAddress
			break
		}
	}
	var clusterClientSet = ClusterClient{}
	if serverAddress != "" {
		kubeconfigGetter := KubeconfigGetterForCluster(c)
		clusterConfig, err := clientcmd.BuildConfigFromKubeconfigGetter(serverAddress, kubeconfigGetter)
		if err != nil {
			return nil, err
		}
		clusterConfig.QPS = KubeAPIQPS
		clusterConfig.Burst = KubeAPIBurst
		clusterClientSet.discoveryClient = discovery.NewDiscoveryClientForConfigOrDie((restclient.AddUserAgent(clusterConfig, UserAgentName)))
		if clusterClientSet.discoveryClient == nil {
			return nil, nil
		}
		clusterClientSet.kubeClient = clientset.NewForConfigOrDie((restclient.AddUserAgent(clusterConfig, UserAgentName)))
		if clusterClientSet.kubeClient == nil {
			return nil, nil
		}
	}
	return &clusterClientSet, err
}

// GetClusterHealthStatus gets the kubernetes cluster health status by requesting "/healthz"
func (self *ClusterClient) GetClusterHealthStatus() *federation_v1alpha1.ClusterStatus {
	clusterStatus := federation_v1alpha1.ClusterStatus{}
	currentTime := unversioned.Now()
	newClusterReadyCondition := federation_v1alpha1.ClusterCondition{
		Type:               federation_v1alpha1.ClusterReady,
		Status:             v1.ConditionTrue,
		Reason:             "ClusterReady",
		Message:            "/healthz responded with ok",
		LastProbeTime:      currentTime,
		LastTransitionTime: currentTime,
	}
	newClusterNotReadyCondition := federation_v1alpha1.ClusterCondition{
		Type:               federation_v1alpha1.ClusterReady,
		Status:             v1.ConditionFalse,
		Reason:             "ClusterNotReady",
		Message:            "/healthz responded without ok",
		LastProbeTime:      currentTime,
		LastTransitionTime: currentTime,
	}
	newNodeOfflineCondition := federation_v1alpha1.ClusterCondition{
		Type:               federation_v1alpha1.ClusterOffline,
		Status:             v1.ConditionTrue,
		Reason:             "ClusterNotReachable",
		Message:            "cluster is not reachable",
		LastProbeTime:      currentTime,
		LastTransitionTime: currentTime,
	}
	newNodeNotOfflineCondition := federation_v1alpha1.ClusterCondition{
		Type:               federation_v1alpha1.ClusterOffline,
		Status:             v1.ConditionFalse,
		Reason:             "ClusterReachable",
		Message:            "cluster is reachable",
		LastProbeTime:      currentTime,
		LastTransitionTime: currentTime,
	}
	body, err := self.discoveryClient.Get().AbsPath("/healthz").Do().Raw()
	if err != nil {
		clusterStatus.Conditions = append(clusterStatus.Conditions, newNodeOfflineCondition)
	} else {
		if !strings.EqualFold(string(body), "ok") {
			clusterStatus.Conditions = append(clusterStatus.Conditions, newClusterNotReadyCondition, newNodeNotOfflineCondition)
		} else {
			clusterStatus.Conditions = append(clusterStatus.Conditions, newClusterReadyCondition)
		}
	}
	return &clusterStatus
}

// GetClusterZones gets the kubernetes cluster zones and region by inspecting labels on nodes in the cluster.
func (self *ClusterClient) GetClusterZones() (zones []string, region string, err error) {
	return getZoneNames(self.kubeClient)
}

// Find the name of the zone in which a Node is running
func getZoneNameForNode(node api.Node) (string, error) {
	for key, value := range node.Labels {
		if key == unversioned.LabelZoneFailureDomain {
			return value, nil
		}
	}
	return "", fmt.Errorf("Zone name for node %s not found. No label with key %s",
		node.Name, unversioned.LabelZoneFailureDomain)
}

// Find the name of the region in which a Node is running
func getRegionNameForNode(node api.Node) (string, error) {
	for key, value := range node.Labels {
		if key == unversioned.LabelZoneRegion {
			return value, nil
		}
	}
	return "", fmt.Errorf("Region name for node %s not found. No label with key %s",
		node.Name, unversioned.LabelZoneRegion)
}

// Find the names of all zones and the region in which we have nodes in this cluster.
func getZoneNames(client *clientset.Clientset) (zones []string, region string, err error) {
	zoneNames := sets.NewString()
	nodes, err := client.Core().Nodes().List(api.ListOptions{})
	if err != nil {
		glog.Errorf("Failed to list nodes while getting zone names: %v", err)
		return nil, "", err
	}
	for i, node := range nodes.Items {
		// TODO: quinton-hoole make this more efficient.
		//       For non-multi-zone clusters the zone will
		//       be identical for all nodes, so we only need to look at one node
		//       For multi-zone clusters we know at build time
		//       which zones are included.  Rather get this info from there, because it's cheaper.
		zoneName, err := getZoneNameForNode(node)
		if err != nil {
			return nil, "", err
		}
		zoneNames.Insert(zoneName)
		if i == 0 {
			region, err = getRegionNameForNode(node)
			if err != nil {
				return nil, "", err
			}
		}
	}
	return zoneNames.List(), region, nil
}
