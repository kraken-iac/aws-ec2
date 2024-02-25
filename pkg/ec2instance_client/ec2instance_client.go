package ec2instanceclient

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type ec2InstanceClient struct {
	ec2Client *ec2.Client
}

func New(ctx context.Context, region string) (*ec2InstanceClient, error) {
	sdkConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := ec2InstanceClient{
		ec2Client: ec2.NewFromConfig(sdkConfig),
	}
	return &client, nil
}

type RunInstancesInput struct {
	MaxCount     int
	MinCount     int
	ImageId      string
	InstanceType string
	Tags         map[string]string
}

func (c ec2InstanceClient) RunInstances(ctx context.Context, params *RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	tags := mapToTags(params.Tags)
	tagSpecs := []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags:         tags,
		},
	}

	output, err := c.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		MaxCount:          aws.Int32(int32(params.MaxCount)),
		MinCount:          aws.Int32(int32(params.MinCount)),
		ImageId:           aws.String(params.ImageId),
		InstanceType:      types.InstanceType(params.InstanceType),
		TagSpecifications: tagSpecs,
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

type FilterOptions struct {
	MatchTags   map[string]string
	MatchStates []types.InstanceStateName
}

func (f FilterOptions) toFilters() []types.Filter {
	filters := make([]types.Filter, 0)
	for k, v := range f.MatchTags {
		filters = append(filters, types.Filter{
			Name:   aws.String("tag:" + k),
			Values: []string{v},
		})
	}
	if len(f.MatchStates) > 0 {
		matchStates := make([]string, len(f.MatchStates))
		for i, v := range f.MatchStates {
			matchStates[i] = string(v)
		}
		filters = append(filters, types.Filter{
			Name:   aws.String("instance-state-name"),
			Values: matchStates,
		})
	}
	return filters
}

func (c ec2InstanceClient) GetInstances(ctx context.Context, filterOptions FilterOptions) ([]types.Instance, error) {
	filters := filterOptions.toFilters()

	var describeInstancesInput ec2.DescribeInstancesInput
	if len(filters) > 0 {
		describeInstancesInput = ec2.DescribeInstancesInput{
			Filters: filters,
		}
	} else {
		describeInstancesInput = ec2.DescribeInstancesInput{}
	}

	describeInstancesoutput, err := c.ec2Client.DescribeInstances(ctx, &describeInstancesInput)
	if err != nil {
		return nil, err
	}

	var instances []types.Instance
	for _, r := range describeInstancesoutput.Reservations {
		instances = append(instances, r.Instances...)
	}
	return instances, nil
}

func (c ec2InstanceClient) TerminateInstances(ctx context.Context, instances []types.Instance) (*ec2.TerminateInstancesOutput, error) {
	instanceIds := make([]string, len(instances))
	for i, inst := range instances {
		instanceIds[i] = *inst.InstanceId
	}
	o, err := c.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: instanceIds})
	return o, err
}

func mapToTags(m map[string]string) []types.Tag {
	tags := make([]types.Tag, len(m))
	i := 0
	for k, v := range m {
		tags[i] = types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		i++
	}
	return tags
}
