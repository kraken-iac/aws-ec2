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
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsv1alpha1 "github.com/kraken-iac/aws-ec2-instance/api/v1alpha1"
	ec2instanceclient "github.com/kraken-iac/aws-ec2-instance/pkg/ec2instance_client"
	krakenv1alpha1 "github.com/kraken-iac/kraken/api/v1alpha1"
)

const (
	ec2InstanceFinalizer string = "aws.kraken-iac.eoinfennessy.com/ec2-instance-finalizer"

	nameTagKey      string = "kraken-name"
	namespaceTagKey string = "kraken-namespace"

	conditionTypeReady string = "Ready"
)

type EC2InstanceClient interface {
	RunInstances(ctx context.Context, params *ec2instanceclient.RunInstancesInput) (*ec2.RunInstancesOutput, error)
	GetInstances(ctx context.Context, filterOptions ec2instanceclient.FilterOptions) ([]types.Instance, error)
	WaitUntilRunning(ctx context.Context, filterOptions ec2instanceclient.FilterOptions, duration time.Duration) error
	TerminateInstances(ctx context.Context, instances []types.Instance) (*ec2.TerminateInstancesOutput, error)
}

// EC2InstanceReconciler reconciles a EC2Instance object
type EC2InstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	EC2InstanceClient
}

//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances/finalizers,verbs=update
//+kubebuilder:rbac:groups=core.kraken-iac.eoinfennessy.com,resources=statedeclarations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EC2Instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *EC2InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile triggered")

	// Fetch ec2Instance resource
	ec2Instance := &awsv1alpha1.EC2Instance{}
	if err := r.Client.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ec2Instance resource not found: Ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to fetch ec2Instance resource: Requeuing")
			return ctrl.Result{}, err
		}
	}

	// Add initial status conditions if not present
	if ec2Instance.Status.Conditions == nil || len(ec2Instance.Status.Conditions) == 0 {
		log.Info("Setting initial status conditions for ec2Instance")
		meta.SetStatusCondition(
			&ec2Instance.Status.Conditions,
			metav1.Condition{
				Type:    conditionTypeReady,
				Status:  metav1.ConditionUnknown,
				Reason:  "Reconciling",
				Message: "Initial reconciliation",
			},
		)

		if err := r.Status().Update(ctx, ec2Instance); err != nil {
			log.Error(err, "Failed to update ec2Instance status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(ec2Instance, ec2InstanceFinalizer) {
		log.Info("Adding finalizer for ec2Instance")
		if ok := controllerutil.AddFinalizer(ec2Instance, ec2InstanceFinalizer); !ok {
			log.Info("Did not add finalizer to ec2Instance as it already exists")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, ec2Instance); err != nil {
			log.Error(err, "Failed to update ec2Instance to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if isMarkedForDeletion(ec2Instance) && controllerutil.ContainsFinalizer(ec2Instance, ec2InstanceFinalizer) {
		log.Info("Performing finalizer operations for ec2Instance before deletion")

		if err := r.doFinalizerOperations(ctx, req, ec2Instance); err != nil {
			log.Error(err, "Failed to perform finalizer operations on ec2Instance")
			return ctrl.Result{}, err
		}

		log.Info("Removing finalizer for EC2Instance")
		if ok := controllerutil.RemoveFinalizer(ec2Instance, ec2InstanceFinalizer); !ok {
			log.Info("Did not remove finalizer from ec2Instance as it is not present")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, ec2Instance); err != nil {
			log.Error(err, "Failed to update ec2Instance after removing finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get running and pending instances matching name and namespace tags
	log.Info("Retrieving EC2 instances", "name", req.Name, "namespace", req.Namespace)
	instances, err := r.EC2InstanceClient.GetInstances(ctx, ec2instanceclient.FilterOptions{
		MatchTags: map[string]string{
			nameTagKey:      req.Name,
			namespaceTagKey: req.Namespace,
		},
		MatchStates: []types.InstanceStateName{
			types.InstanceStateNamePending,
			types.InstanceStateNameRunning,
		},
	})
	if err != nil {
		log.Error(err, "Failed to retrieve EC2 instances")
		meta.SetStatusCondition(
			&ec2Instance.Status.Conditions,
			metav1.Condition{
				Type:    conditionTypeReady,
				Status:  metav1.ConditionUnknown,
				Reason:  "RetrievalFailed",
				Message: "Failed to retrieve EC2 instances",
			},
		)
		return ctrl.Result{Requeue: true}, r.Status().Update(ctx, ec2Instance)
	}

	// TODO: compare all instances to spec and either update (if possible) or terminate those that do not match (update list)

	// Scale down
	if len(instances) > ec2Instance.Spec.MaxCount {
		log.Info("Scaling down EC2 instances")
		terminationCount := len(instances) - ec2Instance.Spec.MaxCount
		if _, err := r.EC2InstanceClient.TerminateInstances(ctx, instances[:terminationCount]); err != nil {
			log.Error(err, "Failed to terminate EC2 instances")
			meta.SetStatusCondition(
				&ec2Instance.Status.Conditions,
				metav1.Condition{
					Type:    conditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "TerminateFailed",
					Message: "Failed to scale down EC2 instances",
				},
			)
			return ctrl.Result{Requeue: true}, r.Update(ctx, ec2Instance)
		}
	}

	// Scale up
	if len(instances) < ec2Instance.Spec.MaxCount {
		log.Info("Scaling up EC2 instances")

		maxCount, minCount := adjustMaxMinInstanceCount(
			len(instances),
			ec2Instance.Spec.MaxCount,
			ec2Instance.Spec.MinCount,
		)

		tags := makeInstanceTags(req, ec2Instance.Spec.Tags)

		o, err := r.EC2InstanceClient.RunInstances(ctx, &ec2instanceclient.RunInstancesInput{
			MaxCount:     maxCount,
			MinCount:     minCount,
			ImageId:      ec2Instance.Spec.ImageId,
			InstanceType: ec2Instance.Spec.InstanceType,
			Tags:         tags,
		})
		if err != nil {
			log.Error(err, "Failed to run instances")
			meta.SetStatusCondition(
				&ec2Instance.Status.Conditions,
				metav1.Condition{
					Type:    conditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "RunFailed",
					Message: "Failed to scale up EC2 instances",
				},
			)
			return ctrl.Result{Requeue: true}, r.Update(ctx, ec2Instance)
		}
		log.Info("Created instances", "instanceCount", len(o.Instances))

		// Wait for pending instances to reach running state
		if err := r.WaitUntilRunning(
			ctx,
			ec2instanceclient.FilterOptions{
				MatchTags: map[string]string{
					nameTagKey:      req.Name,
					namespaceTagKey: req.Namespace,
				},
				MatchStates: []types.InstanceStateName{
					types.InstanceStateNamePending,
					types.InstanceStateNameRunning,
				},
			},
			time.Minute*2,
		); err != nil {
			log.Error(err, "Encountered error waiting for running state")
			meta.SetStatusCondition(
				&ec2Instance.Status.Conditions,
				metav1.Condition{
					Type:    conditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "WaitForRunningError",
					Message: fmt.Sprintf("Encountered error waiting for running state: %s", err),
				},
			)
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, ec2Instance)
		}
	}

	// Retrieve running instances to use in StateDeclaration data
	log.Info("Retrieving running EC2 instances", "name", req.Name, "namespace", req.Namespace)
	instances, err = r.EC2InstanceClient.GetInstances(ctx, ec2instanceclient.FilterOptions{
		MatchTags: map[string]string{
			nameTagKey:      req.Name,
			namespaceTagKey: req.Namespace,
		},
		MatchStates: []types.InstanceStateName{
			types.InstanceStateNameRunning,
		},
	})
	if err != nil {
		log.Error(err, "Failed to retrieve EC2 instances")
		meta.SetStatusCondition(
			&ec2Instance.Status.Conditions,
			metav1.Condition{
				Type:    conditionTypeReady,
				Status:  metav1.ConditionUnknown,
				Reason:  "RetrievalFailed",
				Message: "Failed to retrieve EC2 instances",
			},
		)
		return ctrl.Result{Requeue: true}, r.Status().Update(ctx, ec2Instance)
	}

	// Construct StateDeclaration data
	stateDeclarationData, err := constructStateDeclarationData(*ec2Instance, instances)
	if err != nil {
		log.Error(err, "Could not convert to StateDeclaration data")
		return ctrl.Result{}, err
	}

	// Create or update StateDeclaration
	stateDeclaration := &krakenv1alpha1.StateDeclaration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ec2-" + req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(
		ec2Instance,
		stateDeclaration,
		r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on StateDeclaration")
		return reconcile.Result{}, err
	}

	if result, err := controllerutil.CreateOrUpdate(
		ctx,
		r.Client,
		stateDeclaration,
		func() error {
			stateDeclaration.Spec.Data = *stateDeclarationData
			return nil
		}); err != nil {
		log.Error(err, "Failed to create or update StateDeclaration")
		meta.SetStatusCondition(
			&ec2Instance.Status.Conditions,
			metav1.Condition{
				Type:    conditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "StateDeclarationError",
				Message: fmt.Sprintf("Failed to create/update StateDeclaration: %s", err),
			},
		)
		return ctrl.Result{}, r.Status().Update(ctx, ec2Instance)
	} else {
		log.Info("Created/updated StateDeclaration", "operationResult", string(result))
	}

	// Update status condition type ready to true
	meta.SetStatusCondition(
		&ec2Instance.Status.Conditions,
		metav1.Condition{
			Type:    conditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  "Reconciled",
			Message: "Desired state has been reached",
		},
	)
	// TODO: If no change, add time to requeue for self-heal check to ensure state remains as desired.
	if err := r.Status().Update(ctx, ec2Instance); err != nil {
		log.Error(err, "Failed to update ec2Instance status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *EC2InstanceReconciler) doFinalizerOperations(
	ctx context.Context, req ctrl.Request, ec2Instance *awsv1alpha1.EC2Instance,
) error {
	log := log.FromContext(ctx)

	log.Info("Retrieving EC2 instances")
	instances, err := r.EC2InstanceClient.GetInstances(ctx, ec2instanceclient.FilterOptions{
		MatchTags: map[string]string{
			nameTagKey:      req.Name,
			namespaceTagKey: req.Namespace,
		},
	})
	if err != nil {
		log.Error(err, "Failed to retrieve EC2 instances")
		return err
	}

	log.Info("Terminating EC2 instances")
	if _, err := r.EC2InstanceClient.TerminateInstances(ctx, instances); err != nil {
		log.Error(err, "Failed to terminate EC2 instances")
		return err
	}

	r.Recorder.Event(ec2Instance, "Warning", "Deleting",
		fmt.Sprintf("EC2Instance %s is being deleted from the namespace %s",
			ec2Instance.Name,
			ec2Instance.Namespace),
	)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.EC2Instance{}).
		Owns(&krakenv1alpha1.StateDeclaration{}).
		Complete(r)
}

func adjustMaxMinInstanceCount(current, max, min int) (newMax, newMin int) {
	newMax = max - current
	if min-current < 1 {
		newMin = 1
	} else {
		newMin = min - current
	}
	return newMax, newMin
}

func makeInstanceTags(req reconcile.Request, specTags map[string]string) map[string]string {
	tags := make(map[string]string, len(specTags)+2)
	for tagKey, tagVal := range specTags {
		tags[tagKey] = tagVal
	}
	tags[nameTagKey] = req.Name
	tags[namespaceTagKey] = req.Namespace
	return tags
}

func isMarkedForDeletion(ec2Instance *awsv1alpha1.EC2Instance) bool {
	return ec2Instance.DeletionTimestamp != nil
}

func constructStateDeclarationData(ec2Instance awsv1alpha1.EC2Instance, instances []types.Instance) (*v1.JSON, error) {
	dataMap := make(map[string]interface{})
	dataMap["instances"] = instances
	dataMap["spec"] = ec2Instance.Spec

	dataJSON, err := json.Marshal(dataMap)
	if err != nil {
		return nil, err
	}

	stateDeclarationData := v1.JSON{}
	stateDeclarationData.Raw = dataJSON
	return &stateDeclarationData, nil
}
