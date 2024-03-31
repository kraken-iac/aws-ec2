package mockec2instanceclient

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ec2instanceclient "github.com/kraken-iac/aws-ec2-instance/pkg/ec2instance_client"
)

type MockEC2InstanceClient struct {
	instances []ec2types.Instance
}

func (c MockEC2InstanceClient) RunInstances(ctx context.Context, params *ec2instanceclient.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	newInstances := make([]ec2types.Instance, params.MaxCount)
	for i := range newInstances {
		inst := ec2types.Instance{
			ImageId:      &params.ImageID,
			InstanceType: ec2types.InstanceTypeT2Nano,
		}
		newInstances[i] = inst
	}
	c.appendInstances(newInstances)
	o := ec2.RunInstancesOutput{
		Instances: newInstances,
	}
	return &o, nil
}

func (c MockEC2InstanceClient) GetInstances(ctx context.Context, filterOptions ec2instanceclient.FilterOptions) ([]ec2types.Instance, error) {
	return c.instances, nil
}

func (c MockEC2InstanceClient) WaitUntilRunning(ctx context.Context, filterOptions ec2instanceclient.FilterOptions, duration time.Duration) error {
	return nil
}

func (c MockEC2InstanceClient) TerminateInstances(ctx context.Context, instances []ec2types.Instance) (*ec2.TerminateInstancesOutput, error) {
	c.deleteAllInstances()
	return &ec2.TerminateInstancesOutput{}, nil
}

func (c *MockEC2InstanceClient) appendInstances(instances []ec2types.Instance) {
	c.instances = append(c.instances, instances...)
}

func (c *MockEC2InstanceClient) deleteAllInstances() {
	c.instances = []ec2types.Instance{}
}
