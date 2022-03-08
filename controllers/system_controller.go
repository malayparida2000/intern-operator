/*
Copyright 2022.

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

package controllers

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"

	internv1alpha1 "github.com/malayparida2000/intern-operator/api/v1alpha1"
)

// SystemReconciler reconciles a System object
type SystemReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=intern.malayparida2000,resources=systems,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=intern.malayparida2000,resources=systems/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=intern.malayparida2000,resources=systems/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

func (r *SystemReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Got the instance
	instance := &internv1alpha1.System{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue

			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.

		return ctrl.Result{}, err
	}

	// Checking for the deployment already exists, if not creating a new one

	found := &appsv1.Deployment{}

	err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentSpec(instance)

		err = r.Client.Create(ctx, dep)
		if err != nil {

			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {

		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := instance.Spec.Replicas
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Client.Update(ctx, found)
		if err != nil {

			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the Memcached status with the pod ips
	// List the pods for this deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelsForSystem(instance.Name)),
	}
	if err = r.Client.List(ctx, podList, listOpts...); err != nil {

		return ctrl.Result{}, err
	}
	podIps := getPodIps(podList.Items)

	// Applying status update

	if !reflect.DeepEqual(podIps, instance.Status.PodIps) {
		instance.Status.PodIps = podIps
		err := r.Client.Status().Update(ctx, instance)
		if err != nil {

			return ctrl.Result{}, err
		}
	}
	//Checking any statusupdate error

	return ctrl.Result{}, nil

}

func (r *SystemReconciler) deploymentSpec(instance *internv1alpha1.System) *appsv1.Deployment {

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "intern-pods",
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(instance.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "my-pod",
							Image: "k8s.gcr.io/nginx-slim:0.8",

							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set Memcached instance as the owner and controller
	ctrl.SetControllerReference(instance, dep, r.Scheme)
	return dep
}

func labelsForSystem(name string) map[string]string {
	return map[string]string{"app": "nginx-app"}
}

func getPodIps(pods []corev1.Pod) []string {
	var podIps []string
	for _, pod := range pods {
		podIps = append(podIps, pod.Status.PodIP)
	}
	return podIps
}

// SetupWithManager sets up the controller with the Manager.
func (r *SystemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&internv1alpha1.System{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
