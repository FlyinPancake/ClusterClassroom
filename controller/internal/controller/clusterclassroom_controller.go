/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	classroomv1 "github.com/flyinpancake/clusterclassroom/api/v1"
)

// ClusterClassroomReconciler reconciles a ClusterClassroom object
type ClusterClassroomReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=classroom.flyinpancake.com,resources=clusterclassrooms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=classroom.flyinpancake.com,resources=clusterclassrooms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=classroom.flyinpancake.com,resources=clusterclassrooms/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterClassroom object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ClusterClassroomReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// TODO(user): your logic here
	logger.Info("Reconciling ClusterClassroom")

	var classRoom classroomv1.ClusterClassroom

	if err := r.Client.Get(ctx, req.NamespacedName, &classRoom); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Classroom", "Namespace", classRoom.Status.Namespace, "Phase", classRoom.Status.NamespacePhase)

	// // Add finalizer if it doesn't exist
	// logger.Info("Adding finalizer")

	// if !containsString(classRoom.ObjectMeta.Finalizers, classroomFinalizer) {
	// 	classRoom.ObjectMeta.Finalizers = append(classRoom.ObjectMeta.Finalizers, classroomFinalizer)
	// 	if err := r.Update(ctx, &classRoom); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	if classRoom.Status.Namespace == "" {
		// Create the namespace with the id of the student
		classRoom.Status.Namespace = strings.ToLower(classRoom.Spec.NamespacePrefix + "-" + classRoom.Spec.StudentId)
		classRoom.Status.NamespacePhase = "Scheduled"
		if err := r.Status().Update(ctx, &classRoom); err != nil {
			return ctrl.Result{}, err
		}
	}

	ownerRef := r.ownerRef(classRoom)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating namespace", "Namespace", ns.Name)

			// set the owner reference
			if err := ctrl.SetControllerReference(&classRoom, ns, r.Scheme); err != nil {
				logger.Error(err, "Failed to set owner reference")
				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, ns); err != nil {
				return ctrl.Result{}, err
			}
			classRoom.Status.NamespacePhase = "Created"
			if err := r.Status().Update(ctx, &classRoom); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Create the service account
	if classRoom.Status.ServiceAccount == "" && len(classRoom.Status.Namespace) > 0 {
		logger.Info("Creating service account")
		serviceAccount, err := r.creaetServiceAccount(ctx, classRoom)
		if err != nil {
			return ctrl.Result{}, err
		}
		classRoom.Status.ServiceAccount = serviceAccount
		if err := r.Status().Update(ctx, &classRoom); err != nil {
			return ctrl.Result{}, err
		}

	}

	if classRoom.Status.ServiceAccountRole == "" && len(classRoom.Status.Namespace) > 0 {
		logger.Info("Creating role")

		if err := r.createAdminRole(ctx, classRoom); err != nil {
			return ctrl.Result{}, err
		}

		role, err := r.createRoleBinding(ctx, classRoom)
		if err != nil {
			return ctrl.Result{}, err
		}

		classRoom.Status.ServiceAccountRole = role
		if err := r.Status().Update(ctx, &classRoom); err != nil {
			return ctrl.Result{}, err
		}

		// token, err := r.getServiceAccountToken(ctx, classRoom.Status.Namespace)
		// if err != nil {
		// 	return ctrl.Result{}, err
		// }

		// create kubeconfig
		// _, err = r.createKubeConfig(ctx, classRoom.Status.Namespace, "https://localhost:8443", token)

		// if err != nil {
		// 	return ctrl.Result{}, err
		// }
	}

	// Create the constructor job
	constructor_job, err := r.getOrCreateConstructorJob(ctx, classRoom)
	if err != nil {
		return ctrl.Result{}, err
	}
	// get the latest version of the object
	if err := r.Get(ctx, client.ObjectKeyFromObject(&classRoom), &classRoom); err != nil {
		logger.Error(err, "Failed to get the latest version of the object")
		return ctrl.Result{}, err
	}

	// prepare patch data based on job ctorPhase
	var ctorPhase string
	if constructor_job.Status.Succeeded > 0 {
		ctorPhase = "Completed"
	} else if constructor_job.Status.Failed > 0 {
		ctorPhase = "Failed"
	} else {
		ctorPhase = "Running"
	}

	// create patch
	patch := client.MergeFrom(classRoom.DeepCopy())
	classRoom.Status.ConstructorJobPhase = ctorPhase

	// apply patch
	if err := r.Status().Patch(ctx, &classRoom, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		return ctrl.Result{}, err
	}

	evaluatorJob, err := r.getOrCreateEvaluatorJob(ctx, classRoom)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(&classRoom), &classRoom); err != nil {
		logger.Error(err, "Failed to get the latest version of the object")
		return ctrl.Result{}, err
	}

	var evalPhase string
	// update the job phase
	if evaluatorJob.Status.Succeeded > 0 {
		evalPhase = "Completed"
	} else if evaluatorJob.Status.Failed > 0 {
		evalPhase = "Failed"
	} else {
		evalPhase = "Running"
	}

	patch = client.MergeFrom(classRoom.DeepCopy())
	classRoom.Status.EvaluatorJobPhase = evalPhase

	if err := r.Status().Patch(ctx, &classRoom, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterClassroomReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&classroomv1.ClusterClassroom{}).
		Owns(&corev1.Namespace{}).
		Owns(&batchv1.Job{}).
		// Watches(
		// 	&batchv1.Job{},
		// 	handler.EnqueueRequestForOwner(
		// 		r.Scheme,
		// 		mgr.GetRESTMapper(),
		// 		&classroomv1.ClusterClassroom{},
		// 		handler.OnlyControllerOwner())).
		Complete(r)
}

// func containsString(slice []string, s string) bool {
// 	for _, item := range slice {
// 		if item == s {
// 			return true
// 		}
// 	}
// 	return false
// }

// func removeString(slice []string, s string) []string {
// 	var result []string
// 	for _, item := range slice {
// 		if item != s {
// 			result = append(result, item)
// 		}
// 	}
// 	return result
// }

func (r *ClusterClassroomReconciler) constructorJobName(classRoom classroomv1.ClusterClassroom) string {
	return fmt.Sprintf("%s-constructor-job", classRoom.Status.Namespace)
}

func (r *ClusterClassroomReconciler) getOrCreateConstructorJob(ctx context.Context, classRoom classroomv1.ClusterClassroom) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Name: r.constructorJobName(classRoom), Namespace: classRoom.Status.Namespace}, job)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get constructor job")
		return nil, err
	}

	if err == nil {
		return job, nil
	}

	// Create the job

	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.constructorJobName(classRoom),
			Namespace: classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&classRoom, classRoom.GroupVersionKind()),
			},
		},
		Spec: batchv1.JobSpec{

			BackoffLimit: &classRoom.Spec.Constructor.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: r.adminServiceAccountName(classRoom),
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "constructor",
							Image:   classRoom.Spec.Constructor.Image,
							Command: classRoom.Spec.Constructor.Command,
							Args:    classRoom.Spec.Constructor.Args,
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: classRoom.Status.Namespace,
								},
							},
							ImagePullPolicy: classRoom.Spec.Constructor.ImagePullPolicy,
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "Failed to create constructor job")
		return nil, err
	}

	// Update status

	classRoom.Status.ConstructorJobPhase = "Pending"
	if err := r.Status().Update(ctx, &classRoom); err != nil {
		logger.Error(err, "Failed to update Setup status")
		return nil, err
	}

	return job, nil

}

func (r *ClusterClassroomReconciler) evaluatorJobName(classRoom classroomv1.ClusterClassroom) string {
	return fmt.Sprintf("%s-evaluator-job", classRoom.Status.Namespace)
}

func (r *ClusterClassroomReconciler) getOrCreateEvaluatorJob(ctx context.Context, classRoom classroomv1.ClusterClassroom) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Name: r.evaluatorJobName(classRoom), Namespace: classRoom.Status.Namespace}, job)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get evaluator job")
		return nil, err
	}

	if err == nil {
		return job, nil
	}

	// Create the job

	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.evaluatorJobName(classRoom),
			Namespace: classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&classRoom, classRoom.GroupVersionKind()),
			},
		},
		Spec: batchv1.JobSpec{
			Suspend:      ptr.To(true),
			BackoffLimit: &classRoom.Spec.Evaluator.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: r.adminServiceAccountName(classRoom),
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "evaluator",
							Image:   classRoom.Spec.Evaluator.Image,
							Command: classRoom.Spec.Evaluator.Command,
							Args:    classRoom.Spec.Evaluator.Args,
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: classRoom.Status.Namespace,
								},
							},
							ImagePullPolicy: classRoom.Spec.Evaluator.ImagePullPolicy,
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "Failed to create evaluator job")
		return nil, err
	}

	// Update status

	classRoom.Status.EvaluatorJobPhase = "Pending"
	if err := r.Status().Update(ctx, &classRoom); err != nil {
		logger.Error(err, "Failed to update Setup status")
		return nil, err
	}

	return job, nil
}

func (r *ClusterClassroomReconciler) ownerRef(classRoom classroomv1.ClusterClassroom) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: classRoom.APIVersion,
		Kind:       classRoom.Kind,
		Name:       classRoom.Name,
		UID:        classRoom.UID,
		Controller: ptr.To(true),
	}
}

func (r *ClusterClassroomReconciler) adminServiceAccountName(classRoom classroomv1.ClusterClassroom) string {
	return fmt.Sprintf("%s-admin-sa", classRoom.Status.Namespace)
}

func (r *ClusterClassroomReconciler) creaetServiceAccount(
	ctx context.Context,
	classRoom classroomv1.ClusterClassroom) (string, error) {
	logger := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.adminServiceAccountName(classRoom),
			Namespace: classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(classRoom),
			},
		},
	}

	if err := r.Create(ctx, serviceAccount); err != nil {
		return "", err
	}

	logger.Info("ServiceAccount created", "Name", serviceAccount.Name, "Namespace", serviceAccount.Namespace)

	return serviceAccount.Name, nil
}

func (r *ClusterClassroomReconciler) adminRoleName(classRoom classroomv1.ClusterClassroom) string {
	return fmt.Sprintf("%s-admin", classRoom.Status.Namespace)
}

func (r *ClusterClassroomReconciler) createAdminRole(ctx context.Context, classRoom classroomv1.ClusterClassroom) error {
	logger := log.FromContext(ctx)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.adminRoleName(classRoom),
			Namespace: classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&classRoom, classRoom.GroupVersionKind()),
			},
		},
		// allow everything
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

	// Set the owner reference
	if err := ctrl.SetControllerReference(&classRoom, role, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference")
		return err
	}

	// Create or update the Role if it doesn't exist
	if err := r.Create(ctx, role); err != nil {
		logger.Error(err, "Failed to create Role")
		return err
	}

	return nil
}

func (r *ClusterClassroomReconciler) adminRoleBindingName(classRoom classroomv1.ClusterClassroom) string {
	return fmt.Sprintf("%s-admin-binding", classRoom.Status.Namespace)
}

func (r *ClusterClassroomReconciler) createRoleBinding(
	ctx context.Context,
	classRoom classroomv1.ClusterClassroom) (string, error) {

	logger := log.FromContext(ctx)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.adminRoleBindingName(classRoom),
			Namespace: classRoom.Status.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&classRoom, classRoom.GroupVersionKind()),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      r.adminServiceAccountName(classRoom),
				Namespace: classRoom.Status.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     r.adminRoleName(classRoom),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set the owner reference
	if err := ctrl.SetControllerReference(&classRoom, roleBinding, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference")
		return "", err
	}

	// Create or update the RoleBinding if it doesn't exist
	if err := r.Create(ctx, roleBinding); err != nil {
		logger.Error(err, "Failed to create RoleBinding")
		return "", err
	}

	return roleBinding.Name, nil
}

// func (r *ClusterClassroomReconciler) getServiceAccountToken(ctx context.Context, namespaceName string) (string, error) {
// 	logger := log.FromContext(ctx)

// 	// Get the ServiceAccount
// 	sa := &corev1.ServiceAccount{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "student",
// 			Namespace: namespaceName,
// 		},
// 	}

// 	if err := r.Get(ctx, client.ObjectKeyFromObject(sa), sa); err != nil {
// 		logger.Error(err, "Failed to get ServiceAccount")
// 		return "", err
// 	}

// 	// Find the Secret associated with the ServiceAccount
// 	if len(sa.Secrets) == 0 {
// 		// create a new secret for the service account
// 		secret := &corev1.Secret{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      "student-token",
// 				Namespace: namespaceName,
// 				Annotations: map[string]string{
// 					"kubernetes.io/service-account.name": "student", // Bind to the ServiceAccount
// 				},
// 			},
// 		}

// 		if err := r.Create(ctx, secret); err != nil {
// 			logger.Error(err, "Failed to create secret")
// 			return "", err
// 		}

// 		logger.Info("Secret created", "Name", secret.Name, "Namespace", secret.Namespace)

// 		// Update the ServiceAccount to use the new secret
// 		if err := r.Get(ctx, client.ObjectKeyFromObject(sa), sa); err != nil {
// 			logger.Error(err, "Failed to get ServiceAccount")
// 			return "", err
// 		}
// 	}

// 	// sleep 5 s
// 	time.Sleep(5 * time.Second)

// 	secretName := sa.Secrets[0].Name

// 	secret := &corev1.Secret{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      secretName,
// 			Namespace: namespaceName,
// 		},
// 		Type: corev1.SecretTypeServiceAccountToken,
// 	}

// 	if err := r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
// 		logger.Error(err, "Failed to get Secret")
// 		return "", err
// 	}

// 	// Return the token from the secret
// 	token := string(secret.Data["token"])

// 	return token, nil
// }
