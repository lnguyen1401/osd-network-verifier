# VPC Network verifier PoC

This PoC aims to create an EC2 instance in a particular VPC that runs a network verification test and returns results to the PoC without the need for SSH.

## Running the main PoC script:

Properly define AWS-account specific variables at the top of the script

Ensure the proper locally configured AWS profile is being used:
```
$ export AWS_PROFILE=linh
```

Run the script:
```
$ go run main.go
```

## Building the docker image

Build the network-validator tool with go
```
# Note that CGO_ENABLED=0 is required for cross-platform compatibility
$ CGO_ENABLED=0 go build network-validator.go
```

Build and tag the docker image
```
$ docker build . -t tiwillia/network-validator-test:v0.1
```

Run it to make sure it works
```
$ docker run tiwillia/network-validator-test:v0.1
```

Push the image to docker.io
```
$ docker login docker.io
$ docker push tiwillia/network-validator-test:v0.1
```

## Helpful links:
Execllent example for AWS operations:
  https://github.com/openshift/aws-account-operator/blob/c761b484873f1bcb3591d8cede0583f65e69f391/pkg/controller/account/ec2.go#L206

AWS SDK links:
  https://aws.github.io/aws-sdk-go-v2/docs/code-examples/ec2/createinstance/
  https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2

## Possible Issues with this approach:
- This requires an AMI to be created following the steps in https://aws.amazon.com/premiumsupport/knowledge-center/ec2-linux-rhel7-rhel8-log-user-data/
- The console output only contains the most recent 64Kb of text. If something occurs in the AMI after the userData script is run that produces a 64Kb+ of text, its possible that the userData script output is lost.
- The console output contains much more than just the userData output. Filtering for the userData output and being able to reliably determine the results of the script may be brittle.
