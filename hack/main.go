package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
)

// https://issues.redhat.com/browse/OSD-9044

// General steps for PoC:
// Instantiate configured AWS client
// Generate userData file
// Create EC2 instance
//   This can just use hard-coded config values for now

// Helpful links/examples:
//   Build/destroy EC2 instance: https://github.com/openshift/aws-account-operator/blob/aac458f52f530359c9a9f07f3231ca17b82689fd/pkg/controller/account/ec2.go#L190

var (
	AMIID           string = ""
	InstanceType    string = "t2.micro"
	InstanceCount   int    = 1
	VPCSubnetID     string = ""
	SecurityGroupID string = ""

	AWSRegion string = "us-east-2"
)

func main() {
	// Instantiate configured AWS client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Unable to load config for ec2 client: %s\n", err.Error()))
	}

	// https://docs.aws.amazon.com/sdk-for-go/api/aws/client/#New
	ec2Client := awsclient.New(cfg, info, handlers)

	// Generate the userData file
	userData, err := generateUserData(AWSRegion)
	if err != nil {
		panic(fmt.Sprintf("Unable to generate UserData file: %s\n", err.Error()))
	}

	// Create an ec2 instance
	_, err := CreateEC2Instance(ec2Client, AMIID, InstanceType, InstanceCount, VPCSubnetID, SecurityGroupID, userData)
	if err != nil {
		panic(fmt.Sprintf("Unable to create EC2 Instance: %s\n", err.Error()))
	}
}

func CreateEC2Instance(ec2Client *awsclient, amiID, instanceType, instanceCount, vpcSubnetID, securityGroupId, userdata string) (ec2.Reservation, error) {
	// Build our request, converting the go base types into the pointers required by the SDK
	instanceReq := ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		MaxCount:     aws.Int64(instanceCount),
		MinCount:     aws.Int64(instanceCount),
		InstanceType: aws.String(instanceType),
		// Because we're making this VPC aware, we also have to include a network interface specification
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(true),
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 aws.String(vpcSubnetID),
				Groups: []*string{
					aws.String(securityGroupId),
				},
			},
		},
		UserData: aws.String(userdata),
	}
	// Finally, we make our request
	instanceResp, err := ec2client.RunInstances(&instanceReq)
	if err != nil {
		return ec2.Reservation{}, err
	}

	return *instanceResp, nil
}

func TerminateEC2Instance(client *awsclient, instanceID string) error {
	_, err := client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	if err != nil {
		//log message saying there's been an error while Terminating ec2 instance
		return err
	}

	return nil
}

// UserData script will run various tests on the EC2 instance
// The tests will validate the host/port combinations in the following doc are reachable:
//   https://docs.openshift.com/rosa/rosa_getting_started/rosa-aws-prereqs.html#osd-aws-privatelink-firewall-prerequisites
// It will probably require the following variables:
//   - aws_region

// Generate the userData file
// Note that this function isn't actually necessary if we don't need to provide the command with any variables
// as a static UserData would work just fine.
func generateUserData(awsRegion string) (string, error) {
	var data strings.Builder
	data.WriteString("#!/bin/bash -xe\n")
	data.WriteString(`exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1\n`)

	/* Not necessary I don't think, but if we need env vars defined here is how we can do it:
	fmt.Fprintf(&data, "export AWS_REGION=%s\n", awsRegion)
	fmt.Fprintf(&data, "export CLUSTER_ID=%s\n", clusterId)
	fmt.Fprintf(&data, "export CLUSTER_NAME=%s\n", clusterName)
	fmt.Fprintf(&data, "export SHARD=%s\n", shard)
	fmt.Fprintf(&data, "export BASE_DOMAIN=%s\n", baseDomain)
	*/

	data.WriteString(`echo "USERDATA BEGIN"`)
	data.WriteString(`docker pull docker.io/tiwillia/network-validator-test:v0.1`)
	data.WriteString(`docker run docker.io/tiwillia/network-validator-test:v0.1`)
	data.WriteString(`echo "USERDATA END"`)

	userData := base64.StdEncoding.EncodeToString([]byte(userData.String()))

	return userData, nil
}
