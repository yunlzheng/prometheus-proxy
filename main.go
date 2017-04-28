package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
)

var config *api.Config

var (
	ServiceName = "prometheus"
)

func init() {
	config = api.DefaultConfig()
	config.Address = getOr("CONSUL_SERVER", "consul:8500")
}

func main() {

	err := regstryToConsul()
	if err != nil {
		fmt.Println("Registry agent to consul error....")
		fmt.Println(err.Error())
		return
	}

	router := gin.Default()

	router.GET("/health", healthCheckEndpoint)

	v1 := router.Group("/v1")
	{
		v1.GET("/instances", instancesEndpoint)
		v1.GET("/proxy/:uuid/*action", proxyEndpoint)
	}

	router.Run(":9174")
}

func regstryToConsul() error {
	client, err := api.NewClient(config)
	if err != nil {
		fmt.Println(fmt.Errorf("create consul client failed, error info is %s", err.Error()))
		return err
	}

	metadataServer := getOr("RANCHER_METADATA", "http://rancher-metadata")
	agentIP, _ := get("agent_ip", metadataServer)

	fmt.Println("Start registry agent to consul....")

	return client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:                "prometheus-proxy-" + agentIP,
		Name:              "prometheus-proxy",
		EnableTagOverride: false,
		Address:           agentIP,
		Port:              9174,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: "60s",
			HTTP:     "http://" + agentIP + ":9174/health",
			Interval: "20s",
			Timeout:  "20s",
		},
	})

}

func get(key string, server string) (string, error) {
	resp, err := http.Get(server + "/latest/self/host/" + key)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	data, _ := ioutil.ReadAll(resp.Body)
	return string(data), nil
}

func getOr(env string, value string) string {
	envValue := os.Getenv(env)
	if envValue == "" {
		return value
	}
	return envValue
}

func healthCheckEndpoint(c *gin.Context) {
	c.String(200, "Prometheus Consul Gateway Running.")
}

func instancesEndpoint(c *gin.Context) {
	services, err := fetchConsulServices(ServiceName)
	if err != nil {
		c.String(500, err.Error())
	}
	c.JSON(200, services)
}

func proxyEndpoint(c *gin.Context) {
	uuid := c.Param("uuid")
	proxy := strings.Replace(c.Request.RequestURI, "/v1/proxy/"+uuid, "", 1)
	fmt.Println(proxy)
	services, err := fetchConsulServices(ServiceName)
	if err != nil {
		c.String(500, err.Error())
		return
	}

	service, err := find(services, uuid)
	if err != nil {
		c.String(500, err.Error())
		return
	}

	if service != nil {
		proxyURL := "http://" + service.Service.Address + ":" + fmt.Sprintf("%d", service.Service.Port) + proxy
		fmt.Println("send request to " + proxyURL)
		res, err := http.Get(proxyURL)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		data, _ := ioutil.ReadAll(res.Body)

		m := map[string]interface{}{}
		json.Unmarshal(data, &m)
		c.JSON(200, m)
	} else {
		c.String(500, "Cann't find Service Instance")
	}

}

func find(services []*api.ServiceEntry, serviceUUID string) (*api.ServiceEntry, error) {
	for _, service := range services {
		if service.Service.ID == serviceUUID {
			return service, nil
		}
	}
	return nil, fmt.Errorf("%s", "Cann't find Service Instance")
}

func fetchConsulServices(serviceName string) ([]*api.ServiceEntry, error) {
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	services, _, err := client.Health().Service(ServiceName, "", false, nil)
	return services, err
}
