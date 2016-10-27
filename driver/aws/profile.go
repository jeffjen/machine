package aws

import (
	config "github.com/poddworks/machine/config"

	"encoding/json"
	"io"
	"os"
)

type SubnetProfile struct {
	Az        *string `json:"availability_zone"`
	Cidr      *string `json:"cidr"`
	DefaultAz *bool   `json:"default_for_Az"`
	Id        *string `json:"id"`
	Public    *bool   `json:"public"`
}

type SecurityGroup struct {
	Id   *string `json:"id"`
	Desc *string `json:"description"`
	Name *string `json:"name"`
}

type VPCProfile struct {
	Cidr          *string         `json:"cidr"`
	Id            *string         `json:"id"`
	Subnet        []SubnetProfile `json:"subnet"`
	SecurityGroup []SecurityGroup `json:"security_group"`
}

type AMIProfile struct {
	Arch *string `json:"arch"`
	Desc *string `json:"description"`
	Id   *string `json:"id"`
	Name *string `json:"name"`
}

type KeyPair struct {
	Digest *string `json:"digest"`
	Name   *string `json:"name"`
}

type Profile struct {
	Name    string       `json:"name"`
	Region  string       `json:"region"`
	AccntId string       `json:"account_id"`
	VPC     VPCProfile   `json:"vpc"`
	KeyPair []KeyPair    `json:"key_pair"`
	Ami     []AMIProfile `json:"ami"`
}

type RegionProfile map[string]*Profile

type AWSProfile map[string]RegionProfile

func (a AWSProfile) Load() error {
	origin, err := os.OpenFile(config.Config.AWSProfile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	if err = json.NewDecoder(origin).Decode(&a); err == io.EOF {
		return nil
	} else {
		return err
	}
}

func (a AWSProfile) Dump() error {
	origin, err := os.OpenFile(config.Config.AWSProfile, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	return json.NewEncoder(origin).Encode(a)
}
