package main

// Usage
// $ network-validator --timeout=1s

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"

	"gopkg.in/yaml.v2"
)

var (
	rootCmd = &cobra.Command{
		Use:   "network-validator",
		Short: "Validate network endpoints required for OSD",
		Run:   TestEndpoints,
	}

	endpointList   map[string][]int
	timeout        time.Duration = 500 * time.Millisecond
	config         reachabilityConfig
	configFilePath string = "config.yaml"
)

type reachabilityConfig struct {
	Endpoints []endpoint `yaml:"endpoints"`
}

type endpoint struct {
	Host  string `yaml:"host"`
	Ports []int  `yaml:"ports"`
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFilePath, "config", configFilePath, "Path to configuration file")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", timeout, "Timeout for each dial request made")
}

func main() {
	config = reachabilityConfig{}
	err := config.LoadFromYaml(configFilePath)
	if err != nil {
		err = fmt.Errorf("Unable to reach config file %s: %s", configFilePath, err)
		fmt.Println(err)
		os.Exit(1)
	}
	rootCmd.Execute()
}

func (c *reachabilityConfig) LoadFromYaml(filePath string) error {
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(buf), c)
	if err != nil {
		return err
	}
	return nil
}

func TestEndpoints(cmd *cobra.Command, args []string) {
	// TODO how would we check for wildcard entries like the `.quay.io` entry, where we
	// need to validate any CDN such as `cdn01.quay.io` should be available?
	//  We don't need to. We just best-effort check what we can.

	// TODO we'll need some way to pull in the values for <cluster_id>-<shard>.<aws_region>.amazonaws.com to check
	//      as well as .apps.<cluster_name>.<base_domain>, ec2.<aws_region>.amazonaws.com, and
	//      CLUSTER-NAME-k5bxz-image-registry-<aws_region>-lsiflffxtmfyikx.s3.dualstack.us-east-1.amazonaws.com
	//      and elasticloadbalancing.<aws_region>.amazonaws.com
	//   This is also not necessary as these endpoints will not exist until the cluster is installed anyway.

	failures := []error{}
	for _, e := range config.Endpoints {
		for _, port := range e.Ports {
			err := ValidateReachability(e.Host, port)
			if err != nil {
				fmt.Println(err)
				failures = append(failures, err)
			}
		}
	}

	if len(failures) < 1 {
		fmt.Println("Success!")
		return
	}
	fmt.Println("\nNot all endpoints were reachable:")
	for _, f := range failures {
		fmt.Println(f)
	}
	os.Exit(1)
}

func ValidateReachability(host string, port int) error {
	endpoint := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Validating %s\n", endpoint)
	_, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return fmt.Errorf("Unable to reach %s within specified timeout: %s", endpoint, err)
	}
	return nil
}
