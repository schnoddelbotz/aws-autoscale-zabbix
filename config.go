package main

import (
	"github.com/hashicorp/hcl"
	"io/ioutil"
	"log"
	"strings"
)

type AAZConfig struct {
	ListenerConfig ListenerConfig
	AutoScale      AutoScale
	ZabbixConfig   ZabbixConfig
}

type ListenerConfig struct {
	Address      string `hcl:"Address"`
	TLS_CertPath string `hcl:"TLS_CertPath"`
	TLS_CertKey  string `hcl:"TLS_CertKey"`
	HostsAllow   string `hcl:"HostsAllow"`
}

type AutoScale struct {
	GroupName string `hcl:"GroupName"`
	Region    string `hcl:"Region"`
	AccessKey string `hcl:"AccessKey"`
	SecretKey string `hcl:"SecretKey"`
}

type ZabbixConfig struct {
	//Name  string   `hcl:",key"`
	URL                  string `hcl:"URL"`
	User                 string `hcl:"User"`
	Password             string `hcl:"Password"`
	ScaleDownAction      string `hcl:"ScaleDownAction"`
	RestrictToGroupId    int    `hcl:"RestrictToGroupId"`
	RestrictToTemplateId int    `hcl:"RestrictToTemplateId"`
}

const (
	ScaleDownActionDELETE  = "DELETE"
	ScaleDownActionDISABLE = "DISABLE"
)

func readConfig(filename string) AAZConfig {
	var result AAZConfig
	fileContents, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("FATAL: Cannot read config file ", filename)
	}
	hclParseTree, err := hcl.ParseBytes(fileContents)
	if err != nil {
		log.Fatal("FATAL: Config parser error: ", err)
	}
	if err := hcl.DecodeObject(&result, hclParseTree); err != nil {
		log.Fatal("FATAL: Error decoding config: ", err)
	}
	if result.AutoScale.AccessKey != "" && result.AutoScale.SecretKey != "" {
		ConfigHasAWSKey = true
	}
	if result.ListenerConfig.TLS_CertKey != "" && result.ListenerConfig.TLS_CertPath != "" {
		useTLS = true
	}
	verifyConfig(result)
	return result
}

func verifyConfig(c AAZConfig) {
	if c.AutoScale.GroupName == "" {
		log.Fatal("Missing AutoScale.GroupName in configuration file")
	}
	if c.AutoScale.Region == "" {
		log.Fatal("Missing AutoScale.Region in configuration file")
	}
	for _, v := range []string{c.ZabbixConfig.URL, c.ZabbixConfig.User, c.ZabbixConfig.Password} {
		if v == "" {
			log.Fatal("Missing Zabbix URL, User or Password in configuration file")
		}
	}
	if c.ZabbixConfig.RestrictToGroupId == 0 && c.ZabbixConfig.RestrictToTemplateId == 0 {
		log.Fatal("FATAL: You must restrict Zabbix hosts to Groups or Templates")
	}
	if c.ListenerConfig.HostsAllow == "" {
		log.Print("NOTICE: Access to our service is not restricted (no HostsAllow defined)")
	}
	if c.ZabbixConfig.ScaleDownAction != ScaleDownActionDELETE &&
		c.ZabbixConfig.ScaleDownAction != ScaleDownActionDISABLE {
		log.Fatal("ScaleDownAction must be DELETE or DISABLE")
	}
	if !strings.Contains(c.ListenerConfig.Address, ":") {
		log.Fatal("Listener address must be of format [IP]:Port")
	}
}
