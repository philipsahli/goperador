package gontadorservice

import (
	"context"
	"fmt"
	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	appv1alpha1 "github.com/philipsahli/goperador/pkg/apis/app/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_gontadorservice")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new GontadorService Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGontadorService{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gontadorservice-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource GontadorService
	err = c.Watch(&source.Kind{Type: &appv1alpha1.GontadorService{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner GontadorService
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.GontadorService{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGontadorService{}

// ReconcileGontadorService reconciles a GontadorService object
type ReconcileGontadorService struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a GontadorService object and makes changes based on the state read
// and what is in the GontadorService.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGontadorService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling GontadorService")

	// Fetch the GontadorService instance
	instance := &appv1alpha1.GontadorService{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	pod := newPodForCR(instance)
	svchttp := newServiceForCR(instance, "http", 3000)
	svcgrpc := newServiceForCR(instance, "grpc", 3001)
	route := newRouteForCR(instance)
	controllerutil.SetControllerReference(instance, svchttp, r.scheme)
	controllerutil.SetControllerReference(instance, svcgrpc, r.scheme)
	controllerutil.SetControllerReference(instance, route, r.scheme)

	// Set GontadorService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.client.Create(context.TODO(), svchttp)
		err = r.client.Create(context.TODO(), svcgrpc)
		err = r.client.Create(context.TODO(), route)

		// r.client.List(context.TODO(), request.NamespacedName, route)
		// route := &routev1.Route{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, route)

		// reqLogger.Info(string(route.Spec.Host), "Route.Name", route.Namespace, "route.Name", route.Name)

		podList := &corev1.PodList{}
		labelSelector := labels.SelectorFromSet(labelsForMemcached(instance.Name))
		listOps := &client.ListOptions{Namespace: pod.Namespace, LabelSelector: labelSelector}
		err = r.client.List(context.TODO(), listOps, podList)
		if err != nil {
			reqLogger.Error(err, "Failed to list pods", "Memcached.Namespace", pod.Namespace, "pod.Name", pod.Name)
			return reconcile.Result{}, err
		}
		podNames := getPodNames(podList.Items)

		if !reflect.DeepEqual(podNames, instance.Status.PodNames) {
			instance.Status.PodNames = podNames
			instance.Status.RouteHost = route.Spec.Host
			// instance.Status.RouteName = routeName
			err := r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update Gontador status")
				return reconcile.Result{}, err
			} else {
				reqLogger.Info("Status updated", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)

			}
		}

		if err != nil {
			fmt.Println(err)
		}
		// Pod created successfully - don't requeue
		reqLogger.Info("Work done", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

// env:
//               - name: SYSTEM_ENV
//                 value: dev
//               - name: SYSTEM_INSTANCE
//                 value: demo-stao
//               - name: SERVICE_HOST
//                 valueFrom:
//                   fieldRef:
//                     # apiVersion: v1
//                     fieldPath: spec.nodeName
//               - name: SERVICE_PORT
//                 value: '3000'
//               - name: SERVICE_INSTANCE
//                 valueFrom:
//                   fieldRef:
//                     # apiVersion: v1
//                     fieldPath: metadata.name
//               - name: METRIC_HOST
//                 value: 192.168.1.2
//               - name: METRIC_PORT
//                 value: '2003'
//               - name: REDIS_URL
//                 value: '192.168.1.2:6379'

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *appv1alpha1.GontadorService) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "gontador",
					Image: "172.30.1.1:5000/gontador/gontador",
					Ports: []corev1.ContainerPort{
						{
							Name:          "service",
							ContainerPort: 3000,
							Protocol:      corev1.ProtocolTCP,
						},
					},

					//   ServiceHost: spec.NodeName
					//   ServiceInstance: metadata.name
					Env: []corev1.EnvVar{
						{Name: "SYSTEM_ENV", Value: cr.Spec.EnvName},
						{Name: "SYSTEM_INSTANCE", Value: cr.Spec.InstanceName},
						{Name: "SERVICE_INSTANCE", Value: cr.Name},
						{Name: "METRIC_HOST", Value: cr.Spec.MetricHost},
						{Name: "METRIC_PORT", Value: cr.Spec.MetricPort},
						{Name: "REDIS_URL", Value: cr.Spec.RedisEndpoint},
					},
				},
			},
		},
	}
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newServiceForCR(cr *appv1alpha1.GontadorService, name string, port int32) *corev1.Service {
	labels := map[string]string{
		"app": cr.Name,
	}
	svc :=
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cr.Name + "-svc",
				Namespace: cr.Namespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports: []v1.ServicePort{
					{
						Name:     "gontador-" + name,
						Protocol: v1.ProtocolTCP,
						Port:     port,
					},
				},
			},
		}

	if np != nil {
		svc.Spec.Ports[0]["nodePort"] = 30001
	}
	return svc
}

func newRouteForCR(cr *appv1alpha1.GontadorService) *routev1.Route {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-route",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			// Port: routev1.RoutePort{
			// 	TargetPort: "3000",
			// },
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: cr.Name + "-svc",
			},
		},
	}
}

func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// labelsForMemcached returns the labels for selecting the resources
// belonging to the given memcached CR name.
func labelsForMemcached(name string) map[string]string {
	return map[string]string{"app": name}
}
