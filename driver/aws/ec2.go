package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
)

var (
	DEVICE_NAME = []string{"b", "c", "d", "e", "f", "g", "h", "i"}
)

func ec2_getSubnet(profile *VPCProfile, public bool) (subnetId *string) {
	var collection []*string
	for _, subnet := range profile.Subnet {
		if public && *subnet.Public {
			collection = append(collection, subnet.Id)
		} else if !public && !*subnet.Public {
			collection = append(collection, subnet.Id)
		}
	}
	idx := rand.Intn(len(collection))
	return collection[idx]
}

func ec2_findSecurityGroup(profile *VPCProfile, name ...string) (sgId []*string) {
	sgId = make([]*string, 0)
	for _, grp := range name {
		for _, sgrp := range profile.SecurityGroup {
			if *sgrp.Name == grp {
				sgId = append(sgId, sgrp.Id)
			}
		}
	}
	return
}

func ec2_tagInstance(tags []string, instances []*ec2.Instance) *ec2.CreateTagsInput {
	tagparam := &ec2.CreateTagsInput{
		Tags:      make([]*ec2.Tag, 0, len(tags)),
		Resources: make([]*string, 0, len(instances)),
	}
	for _, inst := range instances {
		tagparam.Resources = append(tagparam.Resources, inst.InstanceId)
	}
	for _, tag := range tags {
		var parts = strings.SplitN(tag, "=", 2)
		if len(parts) == 2 {
			tagparam.Tags = append(tagparam.Tags, &ec2.Tag{
				Key:   aws.String(parts[0]),
				Value: aws.String(parts[1]),
			})
		} else {
			fmt.Fprint(os.Stderr, "Skipping bad tag spec", tag)
		}
	}
	return tagparam
}

func ec2_EbsRoot(size int) (mapping *ec2.BlockDeviceMapping) {
	return &ec2.BlockDeviceMapping{
		DeviceName: aws.String("xvda"),
		Ebs: &ec2.EbsBlockDevice{
			DeleteOnTermination: aws.Bool(true),
			VolumeSize:          aws.Int64(int64(size)),
			VolumeType:          aws.String(ec2.VolumeTypeGp2),
		},
	}
}

func ec2_EbsVols(size ...int) (mapping []*ec2.BlockDeviceMapping) {
	mapping = make([]*ec2.BlockDeviceMapping, 0)
	for i, volSize := range size {
		if volSize <= 0 {
			fmt.Fprint(os.Stderr, "Skipping bad volume size", volSize)
			continue
		}
		if i >= len(DEVICE_NAME) {
			fmt.Fprint(os.Stderr, "You had more volumes then AWS allowed")
			os.Exit(1)
		}
		mapping = append(mapping, &ec2.BlockDeviceMapping{
			DeviceName: aws.String("xvd" + DEVICE_NAME[i]),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int64(int64(volSize)),
				VolumeType:          aws.String(ec2.VolumeTypeGp2),
			},
		})
	}
	return
}

func newEC2Inst(c *cli.Context, profile *Profile) {
	var (
		amiId            = c.String("instance-ami-id")
		keyName          = c.String("instance-key")
		num2Launch       = c.Int("instance-count")
		iamProfile       = c.String("instance-profile")
		instTags         = c.StringSlice("instance-tag")
		instType         = c.String("instance-type")
		instVolRoot      = c.Int("instance-root-size")
		instVols         = c.IntSlice("instance-volume-size")
		isPrivate        = c.Bool("subnet-private")
		subnetId         = c.String("subnet-id")
		networkACLGroups = c.StringSlice("security-group")
	)
	ec2param := &ec2.RunInstancesInput{
		InstanceType:     aws.String(instType),
		MaxCount:         aws.Int64(int64(num2Launch)),
		MinCount:         aws.Int64(1),
		SecurityGroupIds: ec2_findSecurityGroup(&profile.VPC, networkACLGroups...),
	}

	// Step 1: determine the Amazone Machine Image ID
	if amiId != "" {
		ec2param.ImageId = aws.String(amiId)
	} else if len(profile.VPC.Ami) != 0 {
		ec2param.ImageId = profile.VPC.Ami[0].Id
	} else {
		fmt.Fprint(os.Stderr, "Cannot proceed without an AMI")
		os.Exit(1)
	}

	// Step 2: determine keypair to use for remote access
	if keyName != "" {
		ec2param.KeyName = aws.String(keyName)
	} else if len(profile.VPC.KeyPair) != 0 {
		ec2param.KeyName = profile.VPC.KeyPair[0].Name
	} else {
		fmt.Fprint(os.Stderr, "Cannot proceed without SSH keypair")
		os.Exit(1)
	}

	// Step 3: determine EBS Volume configuration
	ec2param.BlockDeviceMappings = make([]*ec2.BlockDeviceMapping, 0)
	if instVolRoot > 0 {
		ec2param.BlockDeviceMappings = append(ec2param.BlockDeviceMappings, ec2_EbsRoot(instVolRoot))
	}
	if len(instVols) > 0 {
		var mapping = ec2_EbsVols(instVols...)
		ec2param.BlockDeviceMappings = append(ec2param.BlockDeviceMappings, mapping...)
	}

	// Step 4: assign IAM role for the EC2 machine
	if strings.HasPrefix(iamProfile, "arn:aws:iam") {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Arn: aws.String(iamProfile),
		}
	} else if iamProfile != "" {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Name: aws.String(iamProfile),
		}
	}

	// Step 5: assign accessibility of EC2 instance by subnet
	if subnetId != "" {
		ec2param.SubnetId = aws.String(subnetId)
	} else {
		ec2param.SubnetId = ec2_getSubnet(&profile.VPC, !isPrivate)
	}

	// Last step: launch + tag instances
	resp, err := svc.RunInstances(ec2param)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(instTags) > 0 {
		_, err = svc.CreateTags(ec2_tagInstance(instTags, resp.Instances))
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}
	fmt.Println("Launched instances...")

	var wg sync.WaitGroup

	// scheduled worker count
	wg.Add(len(resp.Instances))

	for _, inst := range resp.Instances {
		go func(instId *string) {
			defer wg.Done()
			fmt.Println(*instId, "- pending")
			param := &ec2.DescribeInstancesInput{InstanceIds: []*string{instId}}
			if err := svc.WaitUntilInstanceRunning(param); err != nil {
				fmt.Fprint(os.Stderr, *instId, "-", err.Error())
				return
			}
			resp, err := svc.DescribeInstances(param)
			if err != nil {
				fmt.Fprint(os.Stderr, *instId, "-", err.Error())
				return
			}
			for _, rsvp := range resp.Reservations {
				for _, inst := range rsvp.Instances {
					fmt.Println(*inst.InstanceId, "-", *inst.PublicIpAddress, "-", *inst.PrivateIpAddress)
				}
			}
		}(inst.InstanceId)
	}

	// Retrieved all info
	wg.Wait()
}
