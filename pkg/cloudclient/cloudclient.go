package cloudclient

import (
	"context"
	"fmt"

	awscredsv2 "github.com/aws/aws-sdk-go-v2/credentials"
	awscredsv1 "github.com/aws/aws-sdk-go/aws/credentials"
	awsCloudClient "github.com/openshift/osd-network-verifier/pkg/cloudclient/aws"
	gcpCloudClient "github.com/openshift/osd-network-verifier/pkg/cloudclient/gcp"

	"golang.org/x/oauth2/google"
)

// CloudClient defines the interface for a cloud agnostic implementation
// For mocking: mockgen -source=pkg/cloudclient/cloudclient.go -destination=pkg/cloudclient/mock_cloudclient/mock_cloudclient.go
type CloudClient interface {

	// ByoVPCValidator validates the configuration given by the customer
	ByoVPCValidator(ctx context.Context) error

	// ValidateEgress validates that all required targets are reachable from the vpcsubnet
	// required target are defined in https://docs.openshift.com/rosa/rosa_getting_started/rosa-aws-prereqs.html#osd-aws-privatelink-firewall-prerequisites
	ValidateEgress(ctx context.Context, vpcSubnetID, cloudImageID string) error
}

func NewClient(creds interface{}, region string, tags map[string]string) (CloudClient, error) {
	switch c := creds.(type) {
	case awscredsv1.Credentials, awscredsv2.StaticCredentialsProvider:
		return awsCloudClient.NewClient(c, region, tags)
	case *google.Credentials:
		return gcpCloudClient.NewClient(c, region, tags)
	default:
		return nil, fmt.Errorf("unsupported credentials type %T", c)
	}

}
