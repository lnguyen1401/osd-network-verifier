package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
	AMIID           string = "ami-0df9a9ade3c65a1c7"
	InstanceType    string = "t2.micro"
	InstanceCount   int    = 1
	VPCSubnetID     string = "subnet-0af41b2a7187b0df7"
	SecurityGroupID string = "sg-036c3facb0ceb4625"

	AWSRegion string = "us-east-2"
)

func main() {
	// Instantiate configured AWS client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Unable to load config for ec2 client: %s\n", err.Error()))
	}

	// https://aws.github.io/aws-sdk-go-v2/docs/code-examples/ec2/createinstance/
	ec2Client := ec2.NewFromConfig(cfg)

	// Generate the userData file
	userData, err := generateUserData(AWSRegion)
	if err != nil {
		panic(fmt.Sprintf("Unable to generate UserData file: %s\n", err.Error()))
	}

	// Create an ec2 instance
	instance, err := CreateEC2Instance(ec2Client, AMIID, InstanceType, InstanceCount, VPCSubnetID, SecurityGroupID, userData)
	if err != nil {
		panic(fmt.Sprintf("Unable to create EC2 Instance: %s\n", err.Error()))
	}
	instanceID := *instance.Instances[0].InstanceId

	fmt.Printf("Waiting for EC2 instance %s to be running\n", instanceID)
	err = WaitForEC2InstanceCompletion(ec2Client, instanceID)
	if err != nil {
		panic(err)
	}

	// TODO report userdata success/failure and errors
	fmt.Println("TODO: Gather and parse console log output")
}

func CreateEC2Instance(ec2Client *ec2.Client, amiID, instanceType string, instanceCount int, vpcSubnetID, securityGroupId, userdata string) (ec2.RunInstancesOutput, error) {
	// Build our request, converting the go base types into the pointers required by the SDK
	instanceReq := ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		MaxCount:     aws.Int32(int32(instanceCount)),
		MinCount:     aws.Int32(int32(instanceCount)),
		InstanceType: ec2Types.InstanceType(instanceType),
		// Because we're making this VPC aware, we also have to include a network interface specification
		NetworkInterfaces: []ec2Types.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(true),
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 aws.String(vpcSubnetID),
				Groups: []string{
					securityGroupId,
				},
			},
		},
		UserData: aws.String(userdata),
	}
	// Finally, we make our request
	instanceResp, err := ec2Client.RunInstances(context.TODO(), &instanceReq)
	if err != nil {
		return ec2.RunInstancesOutput{}, err
	}

	for _, i := range instanceResp.Instances {
		fmt.Println("Created instance with ID:", *i.InstanceId)
	}

	return *instanceResp, nil
}

// Returns state code as int
func DescribeEC2Instances(client *ec2.Client, instanceID string) (int, error) {
	// States and codes
	// 0 : pending
	// 16 : running
	// 32 : shutting-down
	// 48 : terminated
	// 64 : stopping
	// 80 : stopped
	// 401 : failed
	result, err := client.DescribeInstanceStatus(context.TODO(), &ec2.DescribeInstanceStatusInput{
		InstanceIds: []string{instanceID},
	})

	if err != nil {
		panic(fmt.Sprintf("Errors while describing the instance status: %s\n", err.Error()))
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "UnauthorizedOperation" {
				return 401, err
			}
		}
		return 0, err
	}

	if len(result.InstanceStatuses) > 1 {
		return 0, errors.New("more than one EC2 instance found")
	}

	if len(result.InstanceStatuses) == 0 {
		// Don't return an error here as if the instance is still too new, it may not be
		// retured at all.
		//return 0, errors.New("no EC2 instances found")
		fmt.Printf("Instance %s has no status yet\n", instanceID)
		return 0, nil
	}

	return int(*result.InstanceStatuses[0].InstanceState.Code), nil
}

func WaitForEC2InstanceCompletion(ec2Client *ec2.Client, instanceID string) error {
	//wait for the instance to run
	var descError error
	totalWait := 25 * 60
	currentWait := 1
	// Double the wait time until we reach totalWait seconds
	for totalWait > 0 {
		currentWait = currentWait * 2
		if currentWait > totalWait {
			currentWait = totalWait
		}
		totalWait -= currentWait
		time.Sleep(time.Duration(currentWait) * time.Second)
		var code int
		code, descError = DescribeEC2Instances(ec2Client, instanceID)
		if code == 16 { // 16 represents a successful region initialization
			// Instance is running, break
			break
		} else if code == 401 { // 401 represents an UnauthorizedOperation error
			// Missing permission to perform operations, account needs to fail
			return fmt.Errorf("Missing required permissions for account: %s", descError)
		}

		if descError != nil {
			// Log an error and make sure that instance is terminated
			descErrorMsg := fmt.Sprintf("Could not get EC2 instance state, terminating instance %s", instanceID)

			if descError, ok := descError.(awserr.Error); ok {
				descErrorMsg = fmt.Sprintf("Could not get EC2 instance state: %s, terminating instance %s", descError.Code(), instanceID)
			}

			return fmt.Errorf("%s: %s", descError, descErrorMsg)
		}
	}

	fmt.Printf("EC2 Instance: %s Running\n", instanceID)
	return nil
}

func TerminateEC2Instance(ec2Client *ec2.Client, instanceID string) error {
	input := ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}
	_, err := ec2Client.TerminateInstances(context.TODO(), &input)
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
	data.WriteString("exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1\n")

	/* Not necessary I don't think, but if we need env vars defined here is how we can do it:
	fmt.Fprintf(&data, "export AWS_REGION=%s\n", awsRegion)
	fmt.Fprintf(&data, "export CLUSTER_ID=%s\n", clusterId)
	fmt.Fprintf(&data, "export CLUSTER_NAME=%s\n", clusterName)
	fmt.Fprintf(&data, "export SHARD=%s\n", shard)
	fmt.Fprintf(&data, "export BASE_DOMAIN=%s\n", baseDomain)
	*/

	data.WriteString(`echo "USERDATA BEGIN"` + "\n")
	data.WriteString("docker pull docker.io/tiwillia/network-validator-test:v0.1\n")
	data.WriteString("docker run docker.io/tiwillia/network-validator-test:v0.1\n")
	data.WriteString(`echo "USERDATA END"` + "\n")

	userData := base64.StdEncoding.EncodeToString([]byte(data.String()))

	return userData, nil
}
