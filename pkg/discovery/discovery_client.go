package discovery

import (
	"context"
	"fmt"
	"github.com/chlam4/turbo-goprobe-prometheus/pkg/conf"
	"github.com/chlam4/turbo-goprobe-prometheus/pkg/registration"
	"github.com/golang/glog"
	prometheusHttpClient "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/turbonomic/turbo-go-sdk/pkg/builder"
	"github.com/turbonomic/turbo-go-sdk/pkg/probe"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
	"github.com/turbonomic/turbo-go-sdk/pkg/supplychain"
	"time"
	"github.com/prometheus/common/model"
)

// Discovery Client for the Prometheus Probe
// Implements the TurboDiscoveryClient interface
type PrometheusDiscoveryClient struct {
	TargetConf    *conf.PrometheusTargetConf
	PrometheusApi prometheus.API
}

func NewDiscoveryClient(confFile string) (*PrometheusDiscoveryClient, error) {
	//
	// Parse conf file to create clientConf
	//
	targetConf, err := conf.NewPrometheusTargetConf(confFile)
	if err != nil {
		return nil, err
	}
	glog.Infof("Target Conf %v\n", targetConf)
	//
	// Create a Prometheus client
	//
	promConfig := prometheusHttpClient.Config{Address: targetConf.Address}
	promHttpClient, err := prometheusHttpClient.NewClient(promConfig)
	if err != nil {
		return nil, err
	}

	return &PrometheusDiscoveryClient{
		TargetConf:    targetConf,
		PrometheusApi: prometheus.NewAPI(promHttpClient),
	}, nil
}

// Get the Account Values to create VMTTarget in the turbo server corresponding to this client
func (discClient *PrometheusDiscoveryClient) GetAccountValues() *probe.TurboTargetInfo {
	// Convert all parameters in clientConf to AccountValue list
	targetConf := discClient.TargetConf

	targetId := registration.TargetIdField
	targetIdVal := &proto.AccountValue{
		Key:         &targetId,
		StringValue: &targetConf.Address,
	}

	accountValues := []*proto.AccountValue{
		targetIdVal,
	}

	targetInfo := probe.NewTurboTargetInfoBuilder(registration.ProbeCategory, registration.TargetType,
		registration.TargetIdField, accountValues).Create()

	return targetInfo
}

// Validate the Target
func (discClient *PrometheusDiscoveryClient) Validate(accountValues []*proto.AccountValue) (*proto.ValidationResponse, error) {
	glog.Infof("BEGIN Validation for PrometheusDiscoveryClient %s\n", accountValues)

	validationResponse := &proto.ValidationResponse{}

	glog.Infof("Validation response %s\n", validationResponse)
	return validationResponse, nil
}

// Discover the Target Topology
func (discClient *PrometheusDiscoveryClient) Discover(accountValues []*proto.AccountValue) (*proto.DiscoveryResponse, error) {
	glog.Infof("========= Discovering Prometheus ============= %s\n", accountValues)

	value, err := discClient.PrometheusApi.Query(
		context.Background(),
		"(navigation_timing_response_end_seconds-navigation_timing_request_start_seconds)*1000",
		time.Now(),
	)
	if err != nil {
		glog.Errorf("Error while discovering Prometheus target %s: %s\n", discClient.TargetConf.Address, err)
		// If there is error during discovery, return an ErrorDTO.
		severity := proto.ErrorDTO_CRITICAL
		description := fmt.Sprintf("%v", err)
		errorDTO := &proto.ErrorDTO{
			Severity:    &severity,
			Description: &description,
		}
		discoveryResponse := &proto.DiscoveryResponse{
			ErrorDTO: []*proto.ErrorDTO{errorDTO},
		}
		return discoveryResponse, nil
	}

	propertyNamespace := "DEFAULT"
	propertyName := supplychain.SUPPLY_CHAIN_CONSTANT_IP_ADDRESS
	ipAddress := "10.10.174.90"
	appType := "webdriver"
	var entities []*proto.EntityDTO
	for _, metric := range value.(model.Vector) {
		respTimeCommodity, _ := builder.NewCommodityDTOBuilder(proto.CommodityDTO_RESPONSE_TIME).
			Capacity(100.0).Used(float64(metric.Value)).Create()
		//vcpuCommodity, _ := builder.NewCommodityDTOBuilder(proto.CommodityDTO_VCPU).Used(3.5).Create()
		//vmemCommodity, _ := builder.NewCommodityDTOBuilder(proto.CommodityDTO_VMEM).Used(6.5).Create()

		appDto, err := builder.NewEntityDTOBuilder(proto.EntityDTO_APPLICATION, metric.Metric.String()).
			DisplayName(metric.Metric.String()).
			SellsCommodity(respTimeCommodity).
			//Provider(builder.CreateProvider(proto.EntityDTO_VIRTUAL_MACHINE, "420b1ddf-b89b-69cf-e849-863fe800e546")).
			//BuysCommodities([]*proto.CommodityDTO{vcpuCommodity, vmemCommodity}).
			ApplicationData(&proto.EntityDTO_ApplicationData{
			Type:      &appType,
			IpAddress: &ipAddress,
		}).
			WithProperty(&proto.EntityDTO_EntityProperty{
			Namespace: &propertyNamespace,
			Name:      &propertyName,
			Value:     &ipAddress,
		}).Create()
		if err != nil {
			glog.Errorf("Error building EntityDTO from metric %v: %s", metric, err)
		} else {
			entities = append(entities, appDto)
		}
	}
	discoveryResponse := &proto.DiscoveryResponse{
		EntityDTO: entities,
	}
	glog.Infof("Prometheus discovery response %s\n", discoveryResponse)

	return discoveryResponse, nil
}
