package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	JSONRPC_Method_UserLogin  = "user.login"
	JSONRPC_Method_DeleteHost = "host.delete"
	JSONRPC_Method_UpdateHost = "host.update" // status:1 -> disable
	JSONRPC_Method_GetHost    = "host.get"
	JSONRPC_DefaultVersion    = "2.0"
	JSONRPC_StatusDisableHost = 1
	JSONRPC_StatusEnableHost  = 0
)

type JSONRPC_LoginRequest struct {
	Version string       `json:"jsonrpc"`
	Method  string       `json:"method"` // user.login
	Params  JSONRPC_Auth `json:"params"`
	Id      int          `json:"id"`
}
type JSONRPC_Auth struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// https://www.zabbix.com/documentation/3.2/manual/api/reference/host/get
type JSONRPC_GetHostsRequest struct {
	Version string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  JSONRPC_GetHostsParams `json:"params"`
	Auth    string                 `json:"auth"`
	Id      int                    `json:"id"`
}
type JSONRPC_GetHostsParams struct {
	Output      string  `json:"output"`
	GroupIds    *string `json:"groupids"`
	TemplateIds *string `json:"templateids"`
}
type JSONRPC_GetHostsResponse struct {
	Version string        `json:"jsonrpc"`
	Result  []ZabbixHost  `json:"result"`
	Error   JSONRPC_Error `json:"error"`
	Id      int           `json:"id"`
}

// https://www.zabbix.com/documentation/3.2/manual/api/reference/host/delete
type JSONRPC_DeleteRequest struct {
	Version string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	Auth    string   `json:"auth"`
	Id      int      `json:"id"`
}

// https://www.zabbix.com/documentation/3.2/manual/api/reference/host/update
type JSONRPC_UpdateRequest struct {
	Version string               `json:"jsonrpc"`
	Method  string               `json:"method"`
	Params  JSONRPC_UpdateParams `json:"params"`
	Auth    string               `json:"auth"`
	Id      int                  `json:"id"`
}
type JSONRPC_UpdateParams struct {
	Status int    `json:"status"`
	HostId string `json:"hostid"`
}

type JSONRPC_Response struct {
	Version string        `json:"jsonrpc"`
	Result  string        `json:"result"` // note/fixme: may be array, e.g. hostIds
	Error   JSONRPC_Error `json:"error"`
	Id      int           `json:"id"`
}
type JSONRPC_Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func zabbixGetSession() (string, error) {
	var LoginRequest JSONRPC_LoginRequest
	LoginRequest.Method = JSONRPC_Method_UserLogin
	LoginRequest.Version = JSONRPC_DefaultVersion
	LoginRequest.Params.User = Config.ZabbixConfig.User
	LoginRequest.Params.Password = Config.ZabbixConfig.Password
	jsonRequest, _ := json.Marshal(LoginRequest)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", Config.ZabbixConfig.URL, strings.NewReader(string(jsonRequest)))
	req.Header.Set("Content-Type", "application/json-rpc")
	resp, err := client.Do(req)

	if err != nil {
		return "", errors.New("Failed to post authentication request")
	} else {
		defer resp.Body.Close()
		var result JSONRPC_Response
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Error.Code != 0 {
			return "", errors.New(result.Error.Data)
		}
		return result.Result, nil
	}
}

func zabbixGetHosts() map[string]ZabbixHost {
	session, err := zabbixGetSession()
	if err != nil {
		log.Printf("ERROR: Zabbix authentication failed: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return nil
	}
	var resultHostMap = map[string]ZabbixHost{}
	var GetHostsRequest JSONRPC_GetHostsRequest

	// use nil pointer to make empty fields in marshalled json "null"
	var groupId *string = nil
	var templateId *string = nil
	groupIdValue := strconv.Itoa(Config.ZabbixConfig.RestrictToGroupId)
	if groupIdValue != "0" {
		groupId = &groupIdValue
	}
	templateIdValue := strconv.Itoa(Config.ZabbixConfig.RestrictToTemplateId)
	if templateIdValue != "0" {
		templateId = &templateIdValue
	}

	GetHostsRequest.Auth = session
	GetHostsRequest.Version = JSONRPC_DefaultVersion
	GetHostsRequest.Method = JSONRPC_Method_GetHost
	GetHostsRequest.Params.Output = "extend"
	GetHostsRequest.Params.GroupIds = groupId
	GetHostsRequest.Params.TemplateIds = templateId

	jsonRequest, _ := json.Marshal(GetHostsRequest)
	//log.Print(string(jsonRequest))

	client := &http.Client{}
	req, _ := http.NewRequest("POST", Config.ZabbixConfig.URL, strings.NewReader(string(jsonRequest)))
	req.Header.Set("Content-Type", "application/json-rpc")
	resp, err := client.Do(req)

	if err != nil {
		log.Print("ERROR: Failed to post getHosts request")
		serverStatus.Errors = serverStatus.Errors + 1
		return nil
	} else {
		defer resp.Body.Close()
		var result JSONRPC_GetHostsResponse
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Error.Code != 0 {
			log.Printf("ERROR: zabbixGetHosts failed: %s", result.Error.Data)
			serverStatus.Errors = serverStatus.Errors + 1
			return nil
		}
		for _, host := range result.Result {
			resultHostMap[host.Host] = host
		}
		return resultHostMap
	}
}

func zabbixDeleteHost(hostId string) {
	session, err := zabbixGetSession()
	if err != nil {
		log.Printf("ERROR: Zabbix authentication failed: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}

	hostToDelete := []string{hostId}
	var DeleteRequest JSONRPC_DeleteRequest
	DeleteRequest.Auth = session
	DeleteRequest.Method = JSONRPC_Method_DeleteHost
	DeleteRequest.Params = hostToDelete
	DeleteRequest.Version = JSONRPC_DefaultVersion
	jsonRequest, _ := json.Marshal(DeleteRequest)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", Config.ZabbixConfig.URL, strings.NewReader(string(jsonRequest)))
	req.Header.Set("Content-Type", "application/json-rpc")
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("ERROR: Failed to post delete request for hostId %s", hostId)
		serverStatus.Errors = serverStatus.Errors + 1
	} else {
		defer resp.Body.Close()
		var result JSONRPC_Response
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Error.Code != 0 {
			log.Printf("ERROR: Failed to DELETE host %s: %s", hostId, result.Error.Data)
			serverStatus.Errors = serverStatus.Errors + 1
			return
		}
		log.Printf("SUCCESS: Deleted host %s", hostId)
	}
}

func zabbixDisableHost(hostId string) {
	session, err := zabbixGetSession()
	if err != nil {
		log.Printf("Zabbix authentication failed: %s", err)
		serverStatus.Errors = serverStatus.Errors + 1
		return
	}

	var UpdateRequest JSONRPC_UpdateRequest
	UpdateRequest.Auth = session
	UpdateRequest.Method = JSONRPC_Method_UpdateHost
	UpdateRequest.Params.HostId = hostId
	UpdateRequest.Params.Status = JSONRPC_StatusDisableHost
	UpdateRequest.Version = JSONRPC_DefaultVersion
	jsonRequest, _ := json.Marshal(UpdateRequest)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", Config.ZabbixConfig.URL, strings.NewReader(string(jsonRequest)))
	req.Header.Set("Content-Type", "application/json-rpc")
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("ERROR: Failed to post disable request for hostId %s", hostId)
		serverStatus.Errors = serverStatus.Errors + 1
	} else {
		defer resp.Body.Close()
		var result JSONRPC_Response
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Error.Code != 0 {
			log.Printf("ERROR: Failed to DISABLE host %s: %s", hostId, result.Error.Data)
			serverStatus.Errors = serverStatus.Errors + 1
			return
		}
		log.Printf("SUCCESS: Disabled host %s", hostId)
	}
}

// see also:
// https://github.com/lukecyca/pyzabbix/blob/67b0777365784355c195a2b89c133ed6df7bcfd4/tests/test_api.py#L78
// http://metrics20.org/
// from EC2 instance:
// curl http://169.254.169.254/latest/meta-data/
