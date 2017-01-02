package main

import (
	"flag"
	"fmt"
	"log"
	"time"
)

type ZabbixHost struct {
	HostId string `json:"hostid"`
	Host   string `json:"host"`
	Status string `json:"status"`
}

type AAZStatus struct {
	Errors        int `json:"errors"`
	Warnings      int `json:"warnings"`
	Notifications int `json:"notifications"`
	ZabbixHosts   int `json:"zabbixHosts"`
}

var aazVersion = "0.0.1"
var Config AAZConfig
var ConfigHasAWSKey = false
var useTLS = false

var ConfigFile = flag.String("config", "/etc/aws-autoscale-zabbix.hcl", "AAZ configuration file")
var VersionQuery = flag.Bool("version", false, "get aws-autoscale-zabbix version")
var SkipListener = flag.Bool("skip-listener", false, "one-shot -- do not listen for SNS notifications")
var DryRun = flag.Bool("dry-run", false, "don't kiss, just talk -- only tell what would be changed")

var zabbixHostMap = map[string]ZabbixHost{} // map host(name) -> host "details"
var serverStatus AAZStatus

func main() {
	flag.Parse()

	if *VersionQuery {
		fmt.Println(aazVersion)
		return
	}

	Config = readConfig(*ConfigFile)
	log.Printf("AAZ version %s starting ...", aazVersion)
	if *DryRun {
		log.Print("Running in dry-run mode; will make NO MODIFICATIONS to Zabbix")
	}

	// initialize zabbixHostMap
	log.Printf("Retrieving hosts from Zabbix (GroupId: %d / TemplateId: %d)...",
		Config.ZabbixConfig.RestrictToGroupId, Config.ZabbixConfig.RestrictToTemplateId)
	zabbixHostMap = zabbixGetHosts()
	log.Printf("Found %d matching hosts in Zabbix", len(zabbixHostMap))

	// get AWS group and compare with Zabbix DB
	if ConfigHasAWSKey {
		initalizeHosts()
	} else {
		log.Print("NOTICE: Skipping host initialization as AutoScale group has no IAM user/key defined")
	}

	// enable heartbeat message logging
	go heartBeat()

	// now listen for SNS notifications
	if !*SkipListener {
		startSNSListener()
	}
}

func initalizeHosts() {
	// Compares AWS AutoScalingGroup EC2 instances against Zabbix hosts.
	// Hosts not found in ASG will be "unMonitored" in Zabbix.
	log.Print("Initial sync AWS<->Zabbix: starting")
	log.Printf("Retrieving ASG '%s' members ...", Config.AutoScale.GroupName)
	awsGroupMembers := getAutoScalingGroupMembers(Config.AutoScale.GroupName,
		Config.AutoScale.Region, Config.AutoScale.AccessKey, Config.AutoScale.SecretKey)
	log.Printf("Current ASG members: %s", awsGroupMembers)
	for hostname, host := range zabbixHostMap {
		//log.Printf("%s -> %s\n", hostname, host)
		if contains(awsGroupMembers, hostname) {
			log.Printf("Zabbix host '%s' exists in ASG, too -- KEEPING", hostname)
		} else {
			log.Printf("Zabbix host '%s' does NOT exist in ASG -- REMOVING!", hostname)
			unMonitorHost(host.HostId)
		}
	}
	log.Print("Initial sync AWS<->Zabbix: completed")
	// todo: other way round: INFORM about hosts missing on Zabbix side
}

func unMonitorHost(hostname string) {
	// Removes a host from Zabbix monitoring by DELETING or DISABLING (based on cfg).
	// Respects DryRun bool. Also removes entry from zabbixHostMap.

	// Start by refreshing zabbixHostMap if host not found in map; it may be a "new" auto-(up)scaled host
	if _, ok := zabbixHostMap[hostname]; !ok {
		log.Printf("UnMonitor request for host '%s' triggered Zabbix host map refresh", hostname)
		zabbixHostMap = zabbixGetHosts()
	}

	// (Try to) look up host using map again
	if hostMapEntry, ok := zabbixHostMap[hostname]; ok {
		if *DryRun {
			log.Printf("DRY-RUN: Would now %s Zabbix host '%s'", Config.ZabbixConfig.ScaleDownAction, hostname)
			return
		}
		log.Printf("Trying to %s Zabbix host '%s'", Config.ZabbixConfig.ScaleDownAction, hostname)
		if Config.ZabbixConfig.ScaleDownAction == ScaleDownActionDELETE {
			// drop host from zabbix and zabbixHostMap
			zabbixDeleteHost(hostMapEntry.HostId)
			delete(zabbixHostMap, hostname)
		} else {
			// disable host in zabbix. keep it in zabbixHostMap with new state.
			// to-do: maybe improve hostmapEntry.status -- distinguish in status output
			zabbixDisableHost(hostMapEntry.HostId)
			hostMapEntry.Status = "DISABLED"
			zabbixHostMap[hostname] = hostMapEntry
		}
	} else {
		log.Printf("WARNING: Attempt to unMonitor non-existent Zabbix host '%s'", hostname)
		serverStatus.Warnings = serverStatus.Warnings + 1
	}
}

func heartBeat() {
	// might add some more useful information?
	for {
		time.Sleep(1 * time.Hour)
		log.Printf("Heartbeat -- %d hosts active in Zabbix", len(zabbixHostMap))
	}
}

func contains(s []string, e string) bool {
	// tiny helper for unMonitorHost(): checks whether []s contains e
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
