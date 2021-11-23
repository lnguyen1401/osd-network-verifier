#!/bin/bash -xe
# This is just an example of a working UserData file that outputs to the EC2 console

export CLUSTER_ID="foobar"

# This line copies output directed to the user-data.log to the EC2 instance console
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1

echo "USERDATA-OUTPUT: This is a test 1"
echo "USERDATA-OUTPUT: This is a test 2"
