package networking

const (
	resolvConfigPath = "/etc/resolv.conf"

	networkAbnormalClassifierParamDescription = "network_abnormal_classifier.param.description"

	networkAbnormalDescription = "network_abnormal_classifier.description"

	serviceNetworkInfoService         = "service_abnormal_info_collector.service"
	serviceNetworkInfoEndpoints       = "service_abnormal_info_collector.endpoints"
	serviceNetworkInfoResolvConfig    = "service_abnormal_info_collector.recolvConfig"
	serviceNetworkInfoTelnetService   = "service_abnormal_info_collector.telnetService"
	serviceNetworkInfoTelnetEndpoints = "service_abnormal_info_collector.telnetEndpoints"
	serviceNetworkInfoNodeIPTables    = "service_abnormal_info_collector.nodeIPTables"
	serviceNetworkInfoPodIPTables     = "service_abnormal_info_collector.podIPTables"
)
