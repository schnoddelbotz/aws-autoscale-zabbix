package main

import (
	"encoding/json"
	"fmt"
	"github.com/smartystreets/go-aws-auth"
	"io/ioutil"
	"log"
	"net/http"
)

type AWS_DescribeAutoScalingGroupsResponse struct {
	DescribeAutoScalingGroupsResponse AWS_DescribeAutoScalingGroupsResult `json:"DescribeAutoScalingGroupsResponse"`
	Error                             AWS_API_Error                       `json:"Error"`
}
type AWS_DescribeAutoScalingGroupsResult struct {
	DescribeAutoScalingGroupsResult AWS_AutoScalingGroups `json:"DescribeAutoScalingGroupsResult"`
}
type AWS_AutoScalingGroups struct {
	AutoScalingGroups []AWS_AutoScalingGroup `json:"AutoScalingGroups"`
}
type AWS_AutoScalingGroup struct {
	Instances []AWS_AutoScalingInstance `json:"Instances"`
}
type AWS_AutoScalingInstance struct {
	InstanceId     string `json:"InstanceId"`
	LifecycleState string `json:"LifecycleState"`
}
type AWS_API_Error struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func getAutoScalingGroupMembers(asgName string, region string, accessKey string, secretKey string) []string {
	// https://autoscaling.[REGION].amazonaws.com/?Action=DescribeAutoScalingGroups&
	//        AutoScalingGroupNames.member.1=my-asg&Version=2011-01-01&AUTHPARAMS
	infoURL := fmt.Sprintf("https://autoscaling.%s.amazonaws.com"+
		"/?Action=DescribeAutoScalingGroups&Version=2011-01-01&AutoScalingGroupNames.member.1=%s",
		region, asgName)
	//infoURL = "http://localhost/~jan/describe-as2.json" -- todo: testcases...?

	client := new(http.Client)
	req, err := http.NewRequest("GET", infoURL, nil)
	req.Header.Add("Accept", "application/json")
	awsauth.Sign(req, awsauth.Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
	})

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("FATAL: Cannot GET ASG members: %s", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("FATAL: Cannot read ASG members: %s", err)
	}
	//body := string(bodyBytes)
	//log.Print(body)  <-- todo: add -debug flag?

	var result AWS_DescribeAutoScalingGroupsResponse
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		log.Fatalf("ERROR: Decoding JSON failed: %s", err)
	}

	if result.Error.Code != "" {
		// MUST "panic" here, as all hosts would be removed from Zabbix as result!
		log.Fatalf("FATAL: AWS API Error '%s': %s", result.Error.Code, result.Error.Message)
	}

	// iterate over instances found in JSON response, return list as result
	if len(result.DescribeAutoScalingGroupsResponse.DescribeAutoScalingGroupsResult.AutoScalingGroups) == 0 {
		log.Fatal("FATAL: Sanity check halt -- API did not return ASG infos; check ASG name?")
	}
	var groupMembers = []string{}
	for _, instance := range result.DescribeAutoScalingGroupsResponse.DescribeAutoScalingGroupsResult.AutoScalingGroups[0].Instances {
		groupMembers = append(groupMembers, instance.InstanceId)
	}
	if len(groupMembers) == 0 {
		log.Fatal("FATAL: Sanity check halt -- API returned 0 ASG instances!")
	}
	return groupMembers
}
