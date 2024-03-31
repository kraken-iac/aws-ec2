package controller

import (
	"context"
	"time"

	"github.com/kraken-iac/aws-ec2-instance/api/v1alpha1"
	mockec2instanceclient "github.com/kraken-iac/aws-ec2-instance/pkg/mock_ec2instance_client"
	"github.com/kraken-iac/common/types/option"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("EC2Instance controller", func() {
	const (
		ec2InstanceName      = "test-ec2instance"
		ec2InstanceNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		imageID      = "ami-1234abcd"
		instanceType = "t2.nano"
		count        = 1
	)

	Context("testing EC2Instance reconciliation", func() {
		var ctx context.Context
		var ec2Instance *v1alpha1.EC2Instance
		var ec2InstanceKey types.NamespacedName
		var fakeClient client.WithWatch
		var fakeEC2InstanceClient mockec2instanceclient.MockEC2InstanceClient
		var r *EC2InstanceReconciler

		BeforeEach(func() {
			ctx = context.Background()
			fakeClient = fake.NewClientBuilder().Build()
			fakeEC2InstanceClient = mockec2instanceclient.MockEC2InstanceClient{}
			r = &EC2InstanceReconciler{
				Client:            fakeClient,
				Scheme:            scheme.Scheme,
				EC2InstanceClient: fakeEC2InstanceClient,
			}
			ec2Instance = &v1alpha1.EC2Instance{
				ObjectMeta: v1.ObjectMeta{
					Name:      ec2InstanceName,
					Namespace: ec2InstanceNamespace,
				},
				Spec: v1alpha1.EC2InstanceSpec{
					ImageID: option.String{
						Value: &imageID,
					},
					InstanceType: option.String{
						Value: &instanceType,
					},
					MaxCount: option.Int{
						Value: &count,
					},
					MinCount: option.Int{
						Value: &count,
					},
				},
			}
			ec2InstanceKey = types.NamespacedName{
				Name:      ec2InstanceName,
				Namespace: ec2InstanceNamespace,
			}
		})

		It("should successfully reconcile Spec with concrete values", func() {
			By("creating an EC2Instance resource")
			Expect(r.Client.Create(ctx, ec2Instance)).Should(BeNil())

			createdEC2Instance := &v1alpha1.EC2Instance{}
			Eventually(func() bool {
				err := r.Client.Get(ctx, ec2InstanceKey, createdEC2Instance)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("reconciling the EC2Instance resource")
			req := reconcile.Request{}
			result, err := r.Reconcile(ctx, req)
			Expect(err).Should(BeNil())
			Expect(result.Requeue).Should(BeFalse())
		})
	})
})
