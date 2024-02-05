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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsv1alpha1 "github.com/kraken-iac/aws-ec2-instance/api/v1alpha1"
	ec2instanceclient "github.com/kraken-iac/aws-ec2-instance/pkg/ec2instance_client"
)

const (
	nameTagKey      string = "kraken-name"
	namespaceTagKey string = "kraken-namespace"
)

type EC2InstanceClient interface {
	RunInstances(ctx context.Context, params *ec2instanceclient.RunInstancesInput) (*ec2.RunInstancesOutput, error)
	GetInstances(ctx context.Context, filterOptions ec2instanceclient.FilterOptions) ([]types.Instance, error)
	TerminateInstances(ctx context.Context, instanceIds []string) (*ec2.TerminateInstancesOutput, error)
}

// EC2InstanceReconciler reconciles a EC2Instance object
type EC2InstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	EC2InstanceClient
}

//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances/finalizers,verbs=update

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
	log.Info("reconcile triggered:", "context", ctx, "req", req.String())

	// TODO: Add unknown status condition if none present

	// TODO: Add finalizer if none present

	// TODO: Handle deletion if marked for deletion

	// Fetch ec2Instance resource
	ec2Instance := &awsv1alpha1.EC2Instance{}
	if err := r.Client.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ec2Instance resource not found. Ignoring because it must have been deleted.")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to fetch ec2Instance resource. Requeuing.")
			return ctrl.Result{}, err
		}
	}

	// Get running and pending instances matching name and namespace tags
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
		log.Error(err, "Failed to retrieve EC2 instances.")
		return ctrl.Result{}, err
	}

	// TODO: check if desired state has been achieved. If no updates required, update status and requeue after some time
	// if some instances are pending. Add time to requeue for self-heal check if state is as desired.

	// TODO: compare all instances to spec and either update (if possible) or terminate those that do not match

	// scale down
	if len(instances) > ec2Instance.Spec.MaxCount {
		terminationCount := len(instances) - ec2Instance.Spec.MaxCount
		terminateInstanceIds := make([]string, terminationCount)
		for i, inst := range instances[:terminationCount] {
			terminateInstanceIds[i] = *inst.InstanceId
		}
		_, err := r.EC2InstanceClient.TerminateInstances(ctx, terminateInstanceIds)
		if err != nil {
			log.Error(err, "Failed to terminate EC2 instances.")
			return ctrl.Result{}, err
		}
	}

	// scale up
	if len(instances) < ec2Instance.Spec.MaxCount {
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
			log.Error(err, "could not run instances")
			return ctrl.Result{}, err
		}
		log.Info("started running instances", "instances", o.Instances)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.EC2Instance{}).
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
