/*
Copyright 2019 The Kubernetes Authors.

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

package benchmark

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/kubernetes-sigs/kube-batch/cmd/kube-batch/app"
	"github.com/kubernetes-sigs/kube-batch/cmd/kube-batch/app/options"
	"github.com/kubernetes-sigs/kube-batch/test/integration/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apiextensions-apiserver/test/integration/fixtures"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/kubernetes/pkg/scheduler/algorithmprovider"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	k8stestutil "k8s.io/kubernetes/test/integration/util"
)

// mustSetupScheduler starts the following components:
// - scheduler
// It returns scheduler config factory and destroyFunc which should be used to
// remove resources after finished.
// Notes on rate limiter:
//   - client rate limit is set to 5000.
func mustSetupScheduler() (factory.Configurator, util.ShutdownFunc) {

	tearDown, config, _, err := fixtures.StartDefaultServer(t)
	if err != nil {
		fmt.Errorf(err)
	}
	defer tearDown()

	apiURL, apiShutdown := k8stestutil.StartApiserver()
	clientSet := clientset.NewForConfigOrDie(&restclient.Config{
		Host:          apiURL,
		ContentConfig: restclient.ContentConfig{GroupVersion: &schema.GroupVersion{Group: "", Version: "v1"}},
		QPS:           5000.0,
		Burst:         5000,
	})
	schedulerConfig, schedulerShutdown := util.StartScheduler(clientSet)

	shutdownFunc := func() {
		schedulerShutdown()
		apiShutdown()
	}

	// Before we start the scheduler we delete/remove the fake nodes created by the previous test runs.
	// TODO(shiv): should move this to better location
	DeleteNodesFromPreviousRun(clientSet)
	_ = StartKubeScheduler()
	return schedulerConfig, shutdownFunc

}

// GetClientConfig returns a kubeconfig object which to be passed to a Kubernetes client on initialization.
func GetClientConfig(kubeconfig string) (*restclient.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return restclient.InClusterConfig()
}

func DeleteNodesFromPreviousRun(cs clientset.Interface) {
	//list and delete all the nodes
	nodelist, err := cs.CoreV1().Nodes().List(metav1.ListOptions{})

	if err != nil {
		fmt.Printf("Failed to get nodes list: %v", err)
		os.Exit(-1)
	}

	for _, node := range nodelist.Items {
		if strings.Contains(node.Name, "sample-node-") == true {
			cs.CoreV1().Nodes().Delete(node.Name, &metav1.DeleteOptions{})
			fmt.Println("Delete node ", node.Name)
		}
	}

}

// StartKubeScheduler will start the kube-batch scheduler
func StartKubeScheduler() bool {
	opt := GetDefaultServerOptions()
	if err := opt.CheckOptionOrDie(); err != nil {
		fmt.Errorf("Falied to parse default options %v", err)
		return false
	}
	go func(opt *options.ServerOption) {
		if err := app.Run(opt); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}(opt)
	return true
}

// GetDefaultServerOptions will get the default option for kube-batch
func GetDefaultServerOptions() *options.ServerOption {
	return &options.ServerOption{
		Kubeconfig:       path.Join(os.Getenv("HOME"), clientcmd.RecommendedHomeDir, clientcmd.RecommendedFileName),
		NamespaceAsQueue: true,
		SchedulePeriod:   "1s",
		SchedulerName:    "kube-batch",
	}
}
