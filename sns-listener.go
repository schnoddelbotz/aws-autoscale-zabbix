package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
)

type SNS_Notification struct {
	Type         string `json:"Type"`
	Message      string `json:"Message"`
	SubscribeURL string `json:"SubscribeURL"`
}

type SNS_Message struct {
	Event                string `json:"Event"`
	EC2InstanceId        string `json:"EC2InstanceId"`
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
}

const (
	SNS_EV_Terminate      = "autoscaling:EC2_INSTANCE_TERMINATE"
	SNS_Type_Notification = "Notification"
	SNS_Type_Subscription = "SubscriptionConfirmation"
)

func startSNSListener() {
	var err error
	log.Printf("Now listening for SNS notifications on %s (TLS:%t)", Config.ListenerConfig.Address, useTLS)
	http.HandleFunc("/", snsHandler)
	http.HandleFunc("/status", statusHandler)
	if useTLS {
		err = http.ListenAndServeTLS(Config.ListenerConfig.Address,
			Config.ListenerConfig.TLS_CertPath, Config.ListenerConfig.TLS_CertKey, nil)
	} else {
		err = http.ListenAndServe(Config.ListenerConfig.Address, nil)
	}
	if err != nil {
		log.Fatalf("FATAL: Cannot start SNS listener: %s", err)
	}
}

func snsHandler(w http.ResponseWriter, request *http.Request) {
	// Parses and handles received SNS messages.
	if !hostIsAllowed(request.RemoteAddr) {
		http.Error(w, "Not authorized", 401)
		log.Printf("WARNING: Denied SNS request (401) from %s", request.RemoteAddr)
		serverStatus.Warnings = serverStatus.Warnings + 1
		return
	}
	// todo: sanity-check request content-length
	log.Printf("%s %s %s", request.Host, request.Method, request.URL.EscapedPath()) // todo: -verbose flag?
	bodyBytes, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Print("ERROR: Failed to read request Body: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}
	//body := string(bodyBytes); log.Print(body);// todo: -debug flag?

	// unmarshal whole notification
	var notification SNS_Notification
	err = json.Unmarshal(bodyBytes, &notification)
	if err != nil {
		log.Printf("ERROR: Decoding JSON notification failed: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}
	if notification.Type == SNS_Type_Subscription {
		log.Printf("NOTICE: Subscription confirmation message received. Visit: %s", notification.SubscribeURL)
		return
	}
	if notification.Type != SNS_Type_Notification {
		log.Printf("ERROR: Invalid notification type received: '%s'", notification.Type)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}

	// unescape/unmarshal/sanity check json message contained in notification
	var message SNS_Message
	err = json.Unmarshal([]byte(notification.Message), &message)
	if err != nil {
		log.Printf("ERROR: Decoding JSON message failed: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}
	if message.Event != SNS_EV_Terminate {
		log.Printf("NOTICE: Received non-termination event '%s' (ignored)", message.Event)
		return
	}
	if message.AutoScalingGroupName != Config.AutoScale.GroupName {
		log.Printf("NOTICE: Received message for other ASG '%s' (ignored)", message.AutoScalingGroupName)
		return
	}

	// finally unMonitor host reported in this notification ...
	unMonitorHost(message.EC2InstanceId)
	// ... and update serverStatus accordingly
	serverStatus.Notifications = serverStatus.Notifications + 1
}

func statusHandler(w http.ResponseWriter, request *http.Request) {
	// provide simple server status (errors, warnings, notifications processed,...)
	if !hostIsAllowed(request.RemoteAddr) {
		http.Error(w, "Not authorized", 401)
		log.Printf("WARNING: Denied status request (401) from %s", request.RemoteAddr)
		serverStatus.Warnings = serverStatus.Warnings + 1
		return
	}
	w.Header().Set("Content-Type", "application/json")
	serverStatus.ZabbixHosts = len(zabbixHostMap)
	myJSON, _ := json.Marshal(serverStatus)
	w.Write(myJSON)
}

func hostIsAllowed(remote string) bool {
	clientIP, _, _ := net.SplitHostPort(remote)
	matched, _ := regexp.MatchString(Config.ListenerConfig.HostsAllow, clientIP)
	return matched
}
