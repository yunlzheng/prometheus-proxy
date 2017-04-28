package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
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
	router := gin.Default()

	router.GET("/health", healthCheckEndpoint)

	v1 := router.Group("/v1")
	{
		v1.GET("/instances", instancesEndpoint)
		v1.GET("/proxy/:uuid/*action", proxyEndpoint)
	}

	router.Run(":9174")
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

func ReverseProxy() gin.HandlerFunc {
	target := "localhost:3000"
	return func(c *gin.Context) {
		director := func(req *http.Request) {
			r := c.Request
			req = r
			req.URL.Scheme = "http"
			req.URL.Host = target
			req.Header["my-header"] = []string{r.Header.Get("my-header")}
			// Golang camelcases headers
			delete(req.Header, "My-Header")
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
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